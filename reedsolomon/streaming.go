/**
 * Reed-Solomon Coding over 8-bit values.
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

package reedsolomon

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	v2 "github.com/flew-software/filecrypt"
)

var shardDir = "shard"
var dataShards = flag.Int("data", 10, "Number of shards to split the data into, must be below 257.")
var parShards = flag.Int("par", 2, "Number of parity shards")
var password = "hello"

const fileCryptExtension string = ".fcef"

var outFile = flag.String("out", "", "Alternative output path/file")

var app = v2.App{
	FileCryptExtension: fileCryptExtension,
	Overwrite:          true,
}

func init() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s [-flags] filename.ext\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "Valid flags:\n")
		flag.PrintDefaults()
	}
}
func GetShardDir() (string, error) {

	path, err := os.Getwd()

	if err != nil {
		return "", err

	}
	filepath := filepath.Join(path, "shard")

	return filepath, nil

}
func DeleteShardDir() {

	dir, err := GetShardDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting shard directory: %v\n", err)
		return
	}

	files, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading shard directory: %v\n", err)
		return
	}

	for _, file := range files {
		filePath := filepath.Join(dir, file.Name())
		if err := os.RemoveAll(filePath); err != nil {
			fmt.Fprintf(os.Stderr, "Error deleting file %s: %v\n", filePath, err)
			continue
		}
	}

	fmt.Println("Error : Kidding, It's deleted, I love you :)")
}

func DoEncode(fname string) ([]string, int) {
	var paths []string

	// Create Dir to save shards
	if _, err := os.Stat(shardDir); os.IsNotExist(err) {
		err := os.Mkdir(shardDir, 0755)
		checkErr(err)
	}

	encFile, err := app.Encrypt(fname, v2.Passphrase(password))
	checkErr(err)

	if (*dataShards + *parShards) > 256 {
		fmt.Fprintf(os.Stderr, "Error: sum of data and parity shards cannot exceed 256\n")
		os.Exit(1)
	}

	// Create encoding matrix.
	enc, err := NewStream(*dataShards, *parShards)
	checkErr(err)

	fmt.Println("Opening", encFile)
	f, err := os.Open(encFile)
	checkErr(err)

	instat, err := f.Stat()
	checkErr(err)

	shards := *dataShards + *parShards
	out := make([]*os.File, shards)

	// Create the resulting files.
	_, file := filepath.Split(encFile)

	for i := range out {
		outfn := fmt.Sprintf("%s.%d", file, i)
		fmt.Println("Creating", outfn)
		out[i], err = os.Create(filepath.Join(shardDir, outfn))
		paths = append(paths, out[i].Name())

		checkErr(err)
	}

	// Split into files.
	data := make([]io.Writer, *dataShards)
	for i := range data {
		data[i] = out[i]
	}
	// Do the split
	err = enc.Split(f, data, instat.Size())
	checkErr(err)

	// Close and re-open the files.
	input := make([]io.Reader, *dataShards)

	for i := range data {
		out[i].Close()
		f, err := os.Open(out[i].Name())
		checkErr(err)
		input[i] = f

		defer f.Close()
	}

	// Create parity output writers
	parity := make([]io.Writer, *parShards)
	for i := range parity {
		parity[i] = out[*dataShards+i]
		defer out[*dataShards+i].Close()
	}

	// Calculate the size Per Shard
	fileShard, err := os.Open(paths[1])
	checkErr(err)

	fInfo, err := fileShard.Stat()
	checkErr(err)

	sizePerShard := int(fInfo.Size())

	// Encode parity
	err = enc.Encode(input, parity)
	checkErr(err)
	fmt.Printf("File split into %d data + %d parity shards.\n", *dataShards, *parShards)
	return paths, sizePerShard
}

func DoDecode(fname string, outfn string) {

	// Create Dir to save Decoded file
	if _, err := os.Stat(outfn); os.IsNotExist(err) {
		err := os.Mkdir(outfn, 0755)
		checkErr(err)
	}

	// Create matrix
	enc, err := NewStream(*dataShards, *parShards)
	checkErr(err)

	// Open the inputs
	shards, size, err := openInput(*dataShards, *parShards, fname)
	checkErr(err)

	// Verify the shards
	ok, err := enc.Verify(shards)
	if ok {
		fmt.Println("No reconstruction needed")
	} else {
		fmt.Println("Verification failed. Reconstructing data")
		shards, size, err = openInput(*dataShards, *parShards, fname)
		checkErr(err)
		// Create out destination writers
		out := make([]io.Writer, len(shards))
		for i := range out {
			if shards[i] == nil {
				outfn := filepath.Join(shardDir, fmt.Sprintf("%s.%d", fname, i))
				fmt.Println("Creating", outfn)
				out[i], err = os.Create(outfn)
				checkErr(err)
			}
		}
		err = enc.Reconstruct(shards, out)
		if err != nil {
			fmt.Println("Reconstruct failed -", err)
			os.Exit(1)
		}
		// Close output.
		for i := range out {
			if out[i] != nil {
				err := out[i].(*os.File).Close()
				checkErr(err)
			}
		}
		shards, size, err = openInput(*dataShards, *parShards, fname)
		ok, err = enc.Verify(shards)
		if !ok {
			fmt.Println("Verification failed after reconstruction, data likely corrupted:", err)
			os.Exit(1)
		}
		checkErr(err)

	}
	outfn = filepath.Join(outfn, fname)

	fmt.Println("Writing data to", outfn)
	f, err := os.Create(outfn)
	checkErr(err)

	defer f.Close()

	shards, size, err = openInput(*dataShards, *parShards, fname)
	checkErr(err)

	// We don't know the exact filesize.
	err = enc.Join(f, shards, int64(*dataShards)*size)
	checkErr(err)

}

func openInput(dataShards, parShards int, fname string) (r []io.Reader, size int64, err error) {
	// Create shards and load the data.
	shards := make([]io.Reader, dataShards+parShards)
	for i := range shards {
		infn := filepath.Join(shardDir, fmt.Sprintf("%s.%d", fname, i))
		fmt.Println("Opening", infn)
		f, err := os.Open(infn)
		if err != nil {
			fmt.Println("Error reading file", err)
			shards[i] = nil
			continue
		} else {
			shards[i] = f
		}
		stat, err := f.Stat()
		checkErr(err)
		if stat.Size() > 0 {
			size = stat.Size()
		} else {
			shards[i] = nil
		}
	}
	return shards, size, nil
}

// StreamEncoder is an interface to encode Reed-Salomon parity sets for your data.
// It provides a fully streaming interface, and processes data in blocks of up to 4MB.
//
// For small shard sizes, 10MB and below, it is recommended to use the in-memory interface,
// since the streaming interface has a start up overhead.
//
// For all operations, no readers and writers should not assume any order/size of
// individual reads/writes.
//
// For usage examples, see "stream-encoder.go" and "streamdecoder.go" in the examples
// folder.
type StreamEncoder interface {
	// Encode parity shards for a set of data shards.
	//
	// Input is 'shards' containing readers for data shards followed by parity shards
	// io.Writer.
	//
	// The number of shards must match the number given to NewStream().
	//
	// Each reader must supply the same number of bytes.
	//
	// The parity shards will be written to the writer.
	// The number of bytes written will match the input size.
	//
	// If a data stream returns an error, a StreamReadError type error
	// will be returned. If a parity writer returns an error, a
	// StreamWriteError will be returned.
	Encode(data []io.Reader, parity []io.Writer) error

	// Verify returns true if the parity shards contain correct data.
	//
	// The number of shards must match the number total data+parity shards
	// given to NewStream().
	//
	// Each reader must supply the same number of bytes.
	// If a shard stream returns an error, a StreamReadError type error
	// will be returned.
	Verify(shards []io.Reader) (bool, error)

	// Reconstruct will recreate the missing shards if possible.
	//
	// Given a list of valid shards (to read) and invalid shards (to write)
	//
	// You indicate that a shard is missing by setting it to nil in the 'valid'
	// slice and at the same time setting a non-nil writer in "fill".
	// An index cannot contain both non-nil 'valid' and 'fill' entry.
	// If both are provided 'ErrReconstructMismatch' is returned.
	//
	// If there are too few shards to reconstruct the missing
	// ones, ErrTooFewShards will be returned.
	//
	// The reconstructed shard set is complete, but integrity is not verified.
	// Use the Verify function to check if data set is ok.
	Reconstruct(valid []io.Reader, fill []io.Writer) error

	// Split a an input stream into the number of shards given to the encoder.
	//
	// The data will be split into equally sized shards.
	// If the data size isn't dividable by the number of shards,
	// the last shard will contain extra zeros.
	//
	// You must supply the total size of your input.
	// 'ErrShortData' will be returned if it is unable to retrieve the
	// number of bytes indicated.
	Split(data io.Reader, dst []io.Writer, size int64) (err error)

	// Join the shards and write the data segment to dst.
	//
	// Only the data shards are considered.
	//
	// You must supply the exact output size you want.
	// If there are to few shards given, ErrTooFewShards will be returned.
	// If the total data size is less than outSize, ErrShortData will be returned.
	Join(dst io.Writer, shards []io.Reader, outSize int64) error
}

// StreamReadError is returned when a read error is encountered
// that relates to a supplied stream.
// This will allow you to find out which reader has failed.
type StreamReadError struct {
	Err    error // The error
	Stream int   // The stream number on which the error occurred
}

// Error returns the error as a string
func (s StreamReadError) Error() string {
	return fmt.Sprintf("error reading stream %d: %s", s.Stream, s.Err)
}

// String returns the error as a string
func (s StreamReadError) String() string {
	return s.Error()
}

// StreamWriteError is returned when a write error is encountered
// that relates to a supplied stream. This will allow you to
// find out which reader has failed.
type StreamWriteError struct {
	Err    error // The error
	Stream int   // The stream number on which the error occurred
}

// Error returns the error as a string
func (s StreamWriteError) Error() string {
	return fmt.Sprintf("error writing stream %d: %s", s.Stream, s.Err)
}

// String returns the error as a string
func (s StreamWriteError) String() string {
	return s.Error()
}

// rsStream contains a matrix for a specific
// distribution of datashards and parity shards.
// Construct if using NewStream()
type rsStream struct {
	r *reedSolomon
	o options

	// Shard reader
	readShards func(dst [][]byte, in []io.Reader) error
	// Shard writer
	writeShards func(out []io.Writer, in [][]byte) error

	blockPool sync.Pool
}

// NewStream creates a new encoder and initializes it to
// the number of data shards and parity shards that
// you want to use. You can reuse this encoder.
// Note that the maximum number of data shards is 256.
func NewStream(dataShards, parityShards int, o ...Option) (StreamEncoder, error) {
	if dataShards+parityShards > 256 {
		return nil, ErrMaxShardNum
	}

	r := rsStream{o: defaultOptions}
	for _, opt := range o {
		opt(&r.o)
	}
	// Override block size if shard size is set.
	if r.o.streamBS == 0 && r.o.shardSize > 0 {
		r.o.streamBS = r.o.shardSize
	}
	if r.o.streamBS <= 0 {
		r.o.streamBS = 4 << 20
	}
	if r.o.shardSize == 0 && r.o.maxGoroutines == defaultOptions.maxGoroutines {
		o = append(o, WithAutoGoroutines(r.o.streamBS))
	}

	enc, err := New(dataShards, parityShards, o...)
	if err != nil {
		return nil, err
	}
	r.r = enc.(*reedSolomon)

	r.blockPool.New = func() interface{} {
		return AllocAligned(dataShards+parityShards, r.o.streamBS)
	}
	r.readShards = readShards
	r.writeShards = writeShards
	if r.o.concReads {
		r.readShards = cReadShards
	}
	if r.o.concWrites {
		r.writeShards = cWriteShards
	}

	return &r, err
}

// NewStreamC creates a new encoder and initializes it to
// the number of data shards and parity shards given.
//
// This functions as 'NewStream', but allows you to enable CONCURRENT reads and writes.
func NewStreamC(dataShards, parityShards int, conReads, conWrites bool, o ...Option) (StreamEncoder, error) {
	return NewStream(dataShards, parityShards, append(o, WithConcurrentStreamReads(conReads), WithConcurrentStreamWrites(conWrites))...)
}

func (r *rsStream) createSlice() [][]byte {
	out := r.blockPool.Get().([][]byte)
	for i := range out {
		out[i] = out[i][:r.o.streamBS]
	}
	return out
}

// Encodes parity shards for a set of data shards.
//
// Input is 'shards' containing readers for data shards followed by parity shards
// io.Writer.
//
// The number of shards must match the number given to NewStream().
//
// Each reader must supply the same number of bytes.
//
// The parity shards will be written to the writer.
// The number of bytes written will match the input size.
//
// If a data stream returns an error, a StreamReadError type error
// will be returned. If a parity writer returns an error, a
// StreamWriteError will be returned.
func (r *rsStream) Encode(data []io.Reader, parity []io.Writer) error {
	if len(data) != r.r.dataShards {
		return ErrTooFewShards
	}

	if len(parity) != r.r.parityShards {
		return ErrTooFewShards
	}

	all := r.createSlice()
	defer r.blockPool.Put(all)
	in := all[:r.r.dataShards]
	out := all[r.r.dataShards:]
	read := 0

	for {
		err := r.readShards(in, data)
		switch err {
		case nil:
		case io.EOF:
			if read == 0 {
				return ErrShardNoData
			}
			return nil
		default:
			return err
		}
		out = trimShards(out, shardSize(in))
		read += shardSize(in)
		err = r.r.Encode(all)
		if err != nil {
			return err
		}
		err = r.writeShards(parity, out)
		if err != nil {
			return err
		}
	}
}

// Trim the shards so they are all the same size
func trimShards(in [][]byte, size int) [][]byte {
	for i := range in {
		if len(in[i]) != 0 {
			in[i] = in[i][0:size]
		}
		if len(in[i]) < size {
			in[i] = in[i][:0]
		}
	}
	return in
}

func readShards(dst [][]byte, in []io.Reader) error {
	if len(in) != len(dst) {
		panic("internal error: in and dst size do not match")
	}
	size := -1
	for i := range in {
		if in[i] == nil {
			dst[i] = dst[i][:0]
			continue
		}
		n, err := io.ReadFull(in[i], dst[i])
		// The error is EOF only if no bytes were read.
		// If an EOF happens after reading some but not all the bytes,
		// ReadFull returns ErrUnexpectedEOF.
		switch err {
		case io.ErrUnexpectedEOF, io.EOF:
			if size < 0 {
				size = n
			} else if n != size {
				// Shard sizes must match.
				return ErrShardSize
			}
			dst[i] = dst[i][0:n]
		case nil:
			continue
		default:
			return StreamReadError{Err: err, Stream: i}
		}
	}
	if size == 0 {
		return io.EOF
	}
	return nil
}

func writeShards(out []io.Writer, in [][]byte) error {
	if len(out) != len(in) {
		panic("internal error: in and out size do not match")
	}
	for i := range in {
		if out[i] == nil {
			continue
		}
		n, err := out[i].Write(in[i])
		if err != nil {
			return StreamWriteError{Err: err, Stream: i}
		}
		//
		if n != len(in[i]) {
			return StreamWriteError{Err: io.ErrShortWrite, Stream: i}
		}
	}
	return nil
}

type readResult struct {
	n    int
	size int
	err  error
}

// cReadShards reads shards concurrently
func cReadShards(dst [][]byte, in []io.Reader) error {
	if len(in) != len(dst) {
		panic("internal error: in and dst size do not match")
	}
	var wg sync.WaitGroup
	wg.Add(len(in))
	res := make(chan readResult, len(in))
	for i := range in {
		if in[i] == nil {
			dst[i] = dst[i][:0]
			wg.Done()
			continue
		}
		go func(i int) {
			defer wg.Done()
			n, err := io.ReadFull(in[i], dst[i])
			// The error is EOF only if no bytes were read.
			// If an EOF happens after reading some but not all the bytes,
			// ReadFull returns ErrUnexpectedEOF.
			res <- readResult{size: n, err: err, n: i}

		}(i)
	}
	wg.Wait()
	close(res)
	size := -1
	for r := range res {
		switch r.err {
		case io.ErrUnexpectedEOF, io.EOF:
			if size < 0 {
				size = r.size
			} else if r.size != size {
				// Shard sizes must match.
				return ErrShardSize
			}
			dst[r.n] = dst[r.n][0:r.size]
		case nil:
		default:
			return StreamReadError{Err: r.err, Stream: r.n}
		}
	}
	if size == 0 {
		return io.EOF
	}
	return nil
}

// cWriteShards writes shards concurrently
func cWriteShards(out []io.Writer, in [][]byte) error {
	if len(out) != len(in) {
		panic("internal error: in and out size do not match")
	}
	var errs = make(chan error, len(out))
	var wg sync.WaitGroup
	wg.Add(len(out))
	for i := range in {
		go func(i int) {
			defer wg.Done()
			if out[i] == nil {
				errs <- nil
				return
			}
			n, err := out[i].Write(in[i])
			if err != nil {
				errs <- StreamWriteError{Err: err, Stream: i}
				return
			}
			if n != len(in[i]) {
				errs <- StreamWriteError{Err: io.ErrShortWrite, Stream: i}
			}
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

// Verify returns true if the parity shards contain correct data.
//
// The number of shards must match the number total data+parity shards
// given to NewStream().
//
// Each reader must supply the same number of bytes.
// If a shard stream returns an error, a StreamReadError type error
// will be returned.
func (r *rsStream) Verify(shards []io.Reader) (bool, error) {
	if len(shards) != r.r.totalShards {
		return false, ErrTooFewShards
	}

	read := 0
	all := r.createSlice()
	defer r.blockPool.Put(all)
	for {
		err := r.readShards(all, shards)
		if err == io.EOF {
			if read == 0 {
				return false, ErrShardNoData
			}
			return true, nil
		}
		if err != nil {
			return false, err
		}
		read += shardSize(all)
		ok, err := r.r.Verify(all)
		if !ok || err != nil {
			return ok, err
		}
	}
}

// ErrReconstructMismatch is returned by the StreamEncoder, if you supply
// "valid" and "fill" streams on the same index.
// Therefore it is impossible to see if you consider the shard valid
// or would like to have it reconstructed.
var ErrReconstructMismatch = errors.New("valid shards and fill shards are mutually exclusive")

// Reconstruct will recreate the missing shards if possible.
//
// Given a list of valid shards (to read) and invalid shards (to write)
//
// You indicate that a shard is missing by setting it to nil in the 'valid'
// slice and at the same time setting a non-nil writer in "fill".
// An index cannot contain both non-nil 'valid' and 'fill' entry.
//
// If there are too few shards to reconstruct the missing
// ones, ErrTooFewShards will be returned.
//
// The reconstructed shard set is complete when explicitly asked for all missing shards.
// However its integrity is not automatically verified.
// Use the Verify function to check in case the data set is complete.
func (r *rsStream) Reconstruct(valid []io.Reader, fill []io.Writer) error {
	if len(valid) != r.r.totalShards {
		return ErrTooFewShards
	}
	if len(fill) != r.r.totalShards {
		return ErrTooFewShards
	}

	all := r.createSlice()
	defer r.blockPool.Put(all)
	reconDataOnly := true
	for i := range valid {
		if valid[i] != nil && fill[i] != nil {
			return ErrReconstructMismatch
		}
		if i >= r.r.dataShards && fill[i] != nil {
			reconDataOnly = false
		}
	}

	read := 0
	for {
		err := r.readShards(all, valid)
		if err == io.EOF {
			if read == 0 {
				return ErrShardNoData
			}
			return nil
		}
		if err != nil {
			return err
		}
		read += shardSize(all)
		all = trimShards(all, shardSize(all))

		if reconDataOnly {
			err = r.r.ReconstructData(all) // just reconstruct missing data shards
		} else {
			err = r.r.Reconstruct(all) //  reconstruct all missing shards
		}
		if err != nil {
			return err
		}
		err = r.writeShards(fill, all)
		if err != nil {
			return err
		}
	}
}

// Join the shards and write the data segment to dst.
//
// Only the data shards are considered.
//
// You must supply the exact output size you want.
// If there are to few shards given, ErrTooFewShards will be returned.
// If the total data size is less than outSize, ErrShortData will be returned.
func (r *rsStream) Join(dst io.Writer, shards []io.Reader, outSize int64) error {
	// Do we have enough shards?
	if len(shards) < r.r.dataShards {
		return ErrTooFewShards
	}

	// Trim off parity shards if any
	shards = shards[:r.r.dataShards]
	for i := range shards {
		if shards[i] == nil {
			return StreamReadError{Err: ErrShardNoData, Stream: i}
		}
	}
	// Join all shards
	src := io.MultiReader(shards...)

	// Copy data to dst
	n, err := io.CopyN(dst, src, outSize)
	if err == io.EOF {
		return ErrShortData
	}
	if err != nil {
		return err
	}
	if n != outSize {
		return ErrShortData
	}
	return nil
}

// Split a an input stream into the number of shards given to the encoder.
//
// The data will be split into equally sized shards.
// If the data size isn't dividable by the number of shards,
// the last shard will contain extra zeros.
//
// You must supply the total size of your input.
// 'ErrShortData' will be returned if it is unable to retrieve the
// number of bytes indicated.
func (r *rsStream) Split(data io.Reader, dst []io.Writer, size int64) error {
	if size == 0 {
		return ErrShortData
	}
	if len(dst) != r.r.dataShards {
		return ErrInvShardNum
	}

	for i := range dst {
		if dst[i] == nil {
			return StreamWriteError{Err: ErrShardNoData, Stream: i}
		}
	}

	// Calculate number of bytes per shard.
	perShard := (size + int64(r.r.dataShards) - 1) / int64(r.r.dataShards)

	// Pad data to r.Shards*perShard.
	paddingSize := (int64(r.r.totalShards) * perShard) - size
	data = io.MultiReader(data, io.LimitReader(zeroPaddingReader{}, paddingSize))

	// Split into equal-length shards and copy.
	for i := range dst {
		n, err := io.CopyN(dst[i], data, perShard)
		if err != io.EOF && err != nil {
			return err
		}
		if n != perShard {
			return ErrShortData
		}
	}

	return nil
}

type zeroPaddingReader struct{}

var _ io.Reader = &zeroPaddingReader{}

func (t zeroPaddingReader) Read(p []byte) (n int, err error) {
	n = len(p)
	for i := 0; i < n; i++ {
		p[i] = '\t'
	}
	return n, nil
}
func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
