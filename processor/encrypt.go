package processor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
	"io"
)

// Генерация случайного ключа для AES.
func GenerateRandomAESKey(size int) ([]byte, error) {
	key := make([]byte, size)
	if _, err := io.ReadFull(rand.Reader, key); err != nil { // Читаем случайные байты в ключ
		return nil, err
	}
	return key, nil
}

// Шифрование данных с использованием AES-GCM, добавляя nonce в начало зашифрованного текста.
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

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil { // Заполняем нонс случайными байтами (помогает обеспечить уникальность шифрования для каждого сообщения)
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil) // Шифруем текст и добавляем нонс в начало зашифрованного текста
	return ciphertext, nil
}

// Расшифровка данных, аналогичная шифрованию.
func aesGCMDecrypt(key, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key) // Создаем новый блочный шифр AES с заданным ключом
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block) // Создаем объект GCM (Galois/Counter Mode) для работы с блоком шифрования
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize() // Получаем размер нонса для данного шифра GCM
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short") // Проверяем, достаточно ли длинны зашифрованного текста для нонса
	}
	nonce, ct := ciphertext[:nonceSize], ciphertext[nonceSize:] // Разделяем зашифрованный текст на нонс и сам зашифрованный текст
	plaintext, err := gcm.Open(nil, nonce, ct, nil)             // Расшифровываем текст
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// Шифрует сгенерированный AES-ключ с использованием публичного RSA-ключа получателя.
func rsaEncrypt(pub *rsa.PublicKey, data []byte) ([]byte, error) {
	hash := sha256.New()                                                 // Используем хэш-функцию SHA-256 для генерации случайных данных
	encrypted, err := rsa.EncryptOAEP(hash, rand.Reader, pub, data, nil) // Шифруем данные с использованием алгоритма RSA-OAEP
	if err != nil {
		return nil, err
	}
	return encrypted, nil
}

// Выполняет обратную операцию к rsaEncrypt – расшифровывает данные с использованием приватного ключа.
func rsaDecrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.New()
	decrypted, err := rsa.DecryptOAEP(hash, rand.Reader, priv, data, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
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

// DecryptMessage выполняет расшифровку гибридно зашифрованного сообщения:
// 1. Расшифровывает AES-ключ с использованием RSA-OAEP и приватного ключа получателя.
// 2. Расшифровывает сообщение с использованием AES-GCM с полученным AES-ключом.
func DecryptMessage(encryptedAESKey, encryptedMessage []byte, recipientPrivateKey *rsa.PrivateKey) ([]byte, error) {
	aesKey, err := rsaDecrypt(recipientPrivateKey, encryptedAESKey)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesGCMDecrypt(aesKey, encryptedMessage)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// GenerateKeyPair создает новую пару RSA-ключей с размером 2048 бит
func GenerateKeyPair() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}
