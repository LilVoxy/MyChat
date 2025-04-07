package processor

import (
	"github.com/golang/snappy"
)

func CompressMessage(data []byte) []byte {
	return snappy.Encode(nil, data)
}

func DecompressMessage(data []byte) ([]byte, error) {
	decompressed, err := snappy.Decode(nil, data)
	if err != nil {
		return nil, err
	}
	return decompressed, nil
}
