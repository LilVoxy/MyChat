// processor/key_management.go
package processor

import (
	"crypto/rand"
	"crypto/rsa"
	"io"
)

// GenerateRandomAESKey генерирует случайный ключ для AES шифрования
// размером size байт.
func GenerateRandomAESKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil { // Читаем случайные байты в ключ
		return nil, err
	}
	return key, nil
}

// GenerateKeyPair создает новую пару RSA-ключей с размером 2048 бит
func GenerateKeyPair() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
