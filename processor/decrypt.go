// processor/decrypt.go
package processor

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"errors"
)

// aesGCMDecrypt расшифровывает данные, зашифрованные с помощью AES-GCM.
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

// rsaDecrypt выполняет обратную операцию к rsaEncrypt – расшифровывает данные с использованием приватного ключа.
func rsaDecrypt(priv *rsa.PrivateKey, data []byte) ([]byte, error) {
	hash := sha256.New()
	decrypted, err := rsa.DecryptOAEP(hash, rand.Reader, priv, data, nil)
	if err != nil {
		return nil, err
	}
	return decrypted, nil
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
