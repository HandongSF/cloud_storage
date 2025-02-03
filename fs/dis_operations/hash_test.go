package dis_operations

import "testing"

func TestConvertFileNameForUp(t *testing.T) {
	ConvertFileNameForUP("IMG_6146.JPG.fcef.8")
}

func TestConvertFileNameForDo(t *testing.T) {
	ConvertFileNameForDo("sdfsf", "IMG_6146.JPG.fcef.8")
}
