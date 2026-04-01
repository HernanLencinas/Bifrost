package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
)

// Utilizamos una llave maestra estática para cifrar los valores de configuración.
// En un entorno extremo, esto debería recuperarse de una variable de entorno como BIFROST_MASTER_KEY o un KMS.
var masterKey = []byte("bifrost-super-secret-master-key-")

// Encrypt toma un texto plano, lo cifra usando AES-GCM, y devuelve
// el texto cifrado concatenado con el Nonce en formato Base64.
func Encrypt(plainText string) (string, error) {
	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("aes instance: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm block: %w", err)
	}

	// Creamos un Nonce aleatorio de 12 bytes
	nonce := make([]byte, aesGCM.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("nonce generation: %w", err)
	}

	// Ciframos y hacemos append del texto cifrado al Nonce
	cipherText := aesGCM.Seal(nonce, nonce, []byte(plainText), nil)
	
	// Retornamos el slice resultante en Base64
	return base64.StdEncoding.EncodeToString(cipherText), nil
}

// Decrypt revierte la operación de Encrypt, recibiendo un string en Base64.
func Decrypt(encryptedBase64 string) (string, error) {
	cipherText, err := base64.StdEncoding.DecodeString(encryptedBase64)
	if err != nil {
		return "", fmt.Errorf("base64 decode: %w", err)
	}

	block, err := aes.NewCipher(masterKey)
	if err != nil {
		return "", fmt.Errorf("aes instance: %w", err)
	}

	aesGCM, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("gcm block: %w", err)
	}

	nonceSize := aesGCM.NonceSize()
	if len(cipherText) < nonceSize {
		return "", fmt.Errorf("texto cifrado inválido: longitud muy corta")
	}

	nonce, cipherData := cipherText[:nonceSize], cipherText[nonceSize:]
	
	// Abrimos y validamos el bloque (decripción autenticada)
	plainText, err := aesGCM.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return "", fmt.Errorf("aes-gcm open: %w", err)
	}

	return string(plainText), nil
}
