// processor/encrypt.go
package processor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"io"
)

// aesGCMEncrypt выполняет шифрование данных с использованием AES-GCM,
// добавляя nonce в начало зашифрованного текста.
func aesGCMEncrypt(key []byte, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key) // Создаем новый блочный шифр AES с заданным ключом
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block) // Создаем объект GCM (Galois/Counter Mode) для работы с блоком шифрования
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize()) // Создаем буфер для хранения нонса (инициализационный вектор для каждого сообщения)

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { // Заполняем нонс случайными байтами
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil) // Шифруем текст и добавляем нонс в начало зашифрованного текста
	return ciphertext, nil
}

// rsaEncrypt шифрует сгенерированный AES-ключ с использованием публичного RSA-ключа получателя.
func rsaEncrypt(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	hash := sha256.New()                                                 // Используем хэш-функцию SHA-256
	encrypted, err := rsa.EncryptOAEP(hash, rand.Reader, pub, data, nil) // Шифруем данные с использованием алгоритма RSA-OAEP
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

// EncryptMessage выполняет гибридное шифрование:
// 1. Генерирует случайный AES-ключ (например, 32 байта для AES-256).
// 2. Шифрует исходное сообщение с использованием AES-GCM.
// 3. Шифрует AES-ключ с использованием RSA-OAEP и публичного ключа получателя.
// Функция возвращает зашифрованный AES-ключ и зашифрованное сообщение.
func EncryptMessage(plaintext []byte, recipientPublicKey *rsa.PublicKey) (encryptedAESKey, encryptedMessage []byte, err error) {
	aesKey, err := GenerateRandomAESKey(32)
	if err != nil {
		return nil, nil, err
	}

	encryptedMessage, err = aesGCMEncrypt(aesKey, plaintext)
	if err != nil {
		return nil, nil, err
	}

	encryptedAESKey, err = rsaEncrypt(recipientPublicKey, aesKey)
	if err != nil {
		return nil, nil, err
	}

	return encryptedAESKey, encryptedMessage, nil
}
