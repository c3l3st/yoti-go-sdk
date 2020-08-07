package media

import (
	"encoding/base64"
	"fmt"
)

// Media holds a piece of binary data.
type Media interface {
	// Base64URL is the media encoded as a base64 URL.
	Base64URL() string

	// MIME returns the media's MIME type.
	MIME() string

	// Data returns the media's raw data.
	Data() []byte
}

// NewMedia will create a new appropriate media structure based on the MIME
// type provided. If no suitable structure exists, a Generic one will be used.
func NewMedia(mime string, data []byte) Media {
	switch mime {
	case ImageTypeJPEG:
		return JPEGImage(data)
	case ImageTypePNG:
		return PNGImage(data)
	default:
		return NewGeneric(mime, data)
	}
}

type Generic struct {
	mime string
	data []byte
}

func NewGeneric(mime string, data []byte) Generic {
	return Generic{
		mime: mime,
		data: data,
	}
}

func (g Generic) MIME() string {
	return g.mime
}

func (g Generic) Base64URL() string {
	return base64URL(g.MIME(), g.data)
}

func (g Generic) Data() []byte {
	return g.data
}

func base64URL(mimeType string, data []byte) string {
	base64EncodedImage := base64.StdEncoding.EncodeToString(data)
	return fmt.Sprintf("data:%s;base64,%s", mimeType, base64EncodedImage)
}
