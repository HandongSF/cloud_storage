/**
 * Reed-Solomon Coding over 8-bit values.
 *
 * Copyright 2015, Klaus Post
 * Copyright 2015, Backblaze, Inc.
 */

// Package reedsolomon enables Erasure Coding in Go
//
// * File size.
// * The number of data/parity shards.
// * HASH of each shard.
// * Order of the shards.
//
// If you save these properties, you should abe able to detect file corruption
// in a shard and be able to reconstruct your data if you have the needed number of shards left.

package reedsolomon

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"io"

	v2 "github.com/flew-software/filecrypt"
	"github.com/klauspost/reedsolomon"
)

var shardDir = "shard"
var dataShards = flag.Int("data", 10, "Number of shards to split the data into, must be below 257.")
var parShards = flag.Int("par", 2, "Number of parity shards")
var outDir = flag.String("out", shardDir, "Alternative output directory")
var password = "hello"

// var fileLocation = "./"

const fileCryptExtension string = ".fcef"

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

	if _, err := os.Stat(*outDir); os.IsNotExist(err) {
		err := os.Mkdir(*outDir, 0755)
		checkErr(err)
	}

	// encFile, err := app.Encrypt(fname, v2.Passphrase(password))
	// checkErr(err)
	encFile := fname

	if (*dataShards + *parShards) > 256 {
		fmt.Fprintf(os.Stderr, "Error: sum of data and parity shards cannot exceed 256\n")
		os.Exit(1)
	}

	// Create encoding matrix.
	enc, err := reedsolomon.NewStream(*dataShards, *parShards)
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

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) != 1 {
		fmt.Fprintf(os.Stderr, "Error: No input filename given\n")
		flag.Usage()
		os.Exit(1)
	}

	fname := args[0]
	paths, fileSize := DoEncode(fname)
	fmt.Println("file size is", fileSize)

	filepathshard, _ := GetShardDir()
	fmt.Println("shard is at ", filepathshard)

	fmt.Println("===== 결과 ======")
	for i, path := range paths {
		fmt.Println(i, ": ", path)
	}
}

func checkErr(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %s", err.Error())
		os.Exit(2)
	}
}
