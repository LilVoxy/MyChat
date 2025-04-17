// database/message_crypto.go
package database

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// Секретный ключ для шифрования/расшифровки сообщений в БД
// В реальном приложении должен храниться в защищенном месте (например, в переменных окружения)
var dbEncryptionKey = []byte("this-is-32-byte-key-for-AES-GCM!") // Ровно 32 байта

// encryptForDB шифрует сообщение перед сохранением в базу данных
func encryptForDB(plaintext string) (string, error) {
	// Преобразуем текст в байты
	plaintextBytes := []byte(plaintext)

	// Создаем AES-шифр с нашим ключом
	block, err := aes.NewCipher(dbEncryptionKey)
	if err != nil {
		return "", err
	}

	// Создаем GCM (Galois/Counter Mode)
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Создаем nonce (number used once) - должен быть уникальным для каждого сообщения
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// Шифруем данные
	ciphertext := gcm.Seal(nonce, nonce, plaintextBytes, nil)

	// Кодируем в base64 для удобного хранения в БД
	encoded := base64.StdEncoding.EncodeToString(ciphertext)

	return encoded, nil
}

// decryptFromDB расшифровывает сообщение, полученное из базы данных
func decryptFromDB(encryptedText string) (string, error) {
	// Декодируем base64
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedText)
	if err != nil {
		return "", err
	}

	// Создаем AES-шифр с нашим ключом
	block, err := aes.NewCipher(dbEncryptionKey)
	if err != nil {
		return "", err
	}

	// Создаем GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	// Проверяем длину шифротекста
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Извлекаем nonce и шифротекст
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]

	// Расшифровываем
	plaintextBytes, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintextBytes), nil
}
