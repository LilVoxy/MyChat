package processor

import (
	"crypto/rsa"
)

// ProcessOutboundMessage объединяет два этапа обработки:
// 1. Сжатие исходного сообщения с использованием Snappy (функция CompressMessage из compress.go)
// 2. Гибридное шифрование с использованием AES-GCM для шифрования сжатого сообщения и RSA-OAEP для защиты AES-ключа.
// При успешном выполнении возвращаются зашифрованный AES-ключ и зашифрованное сообщение.
func ProcessOutboundMessage(plaintext []byte, recipientPublicKey *rsa.PublicKey) (encryptedAESKey, encryptedMessage []byte, err error) {
	// Сжатие сообщения
	compressed := CompressMessage(plaintext)
	
	// Гибридное шифрование: шифруем сжатое сообщение
	encryptedAESKey, encryptedMessage, err = EncryptMessage(compressed, recipientPublicKey)
	if err != nil {
		return nil, nil, err
	}
	return encryptedAESKey, encryptedMessage, nil
}

// ProcessInboundMessage выполняет обратный процесс обработки входящего сообщения:
// 1. Расшифровывает зашифрованный AES-ключ с помощью RSA-OAEP и приватного ключа получателя,
// 2. Дешифрует сообщение с использованием AES-GCM с полученным AES-ключом,
// 3. Распаковывает расшифрованное (сжатое) сообщение с использованием Snappy (функция DecompressMessage из compress.go).
// Возвращается исходное сообщение (plaintext).
func ProcessInboundMessage(encryptedAESKey, encryptedMessage []byte, recipientPrivateKey *rsa.PrivateKey) (plaintext []byte, err error) {
	// Расшифровываем сообщение гибридным способом: сначала получаем сжатый текст
	compressed, err := DecryptMessage(encryptedAESKey, encryptedMessage, recipientPrivateKey)
	if err != nil {
		return nil, err
	}
	// Распаковка сжатого сообщения
	plaintext, err = DecompressMessage(compressed)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}
