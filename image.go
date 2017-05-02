package yoti

type ImageType int

const (
	ImageType_Jpeg ImageType = 1 + iota
	ImageType_Png
)

type Image struct {
	Type ImageType
	Data []byte
}

func (image *Image) GetContentType() string {
	switch image.Type {
	case ImageType_Jpeg:
		return "image/jpeg"

	case ImageType_Png:
		return "image/png"

	default:
		return ""
	}
}

func (image *Image) URL() string {
	return "data:application/octet-stream;" + image.GetContentType() + ";," + string(image.Data)
}
