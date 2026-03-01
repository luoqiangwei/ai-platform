package utils

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"time"
)

// HashMD5 returns the MD5 checksum of the data.
func HashMD5(data string) string {
	hash := md5.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HashSHA1 returns the SHA-1 checksum of the data.
func HashSHA1(data string) string {
	hash := sha1.Sum([]byte(data))
	return hex.EncodeToString(hash[:])
}

// HashSHA256 returns the SHA-256 checksum of the data.
func HashSHA256(data string) string {
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// Base64Encode encodes a string to base64.
func Base64Encode(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// Base64Decode decodes a base64 string.
func Base64Decode(data string) (string, error) {
	decoded, err := base64.StdEncoding.DecodeString(data)
	return string(decoded), err
}

// EncryptAES encrypts plaintext using AES-GCM. Key must be 16, 24, or 32 bytes.
func EncryptAES(key []byte, plaintext string) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return hex.EncodeToString(ciphertext), nil
}

func DecryptAES(key []byte, ciphertextHex string) (string, error) {
	data, err := hex.DecodeString(ciphertextHex)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}
	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// Note: RSA implementation would require generating keys and using x509 blocks.
// To keep the scaffold concise, it is omitted here but can be added using the "crypto/rsa" package.
// GenerateRSAKeys creates a new pair of RSA keys.
func GenerateRSAKeys(bits int) (privKey []byte, pubKey []byte, err error) {
	// Generate key pair
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, nil, err
	}

	// Encode private key to PEM
	privBytes := x509.MarshalPKCS1PrivateKey(privateKey)
	privBlock := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: privBytes}
	privKey = pem.EncodeToMemory(privBlock)

	// Encode public key to PEM
	pubBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}
	pubBlock := &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}
	pubKey = pem.EncodeToMemory(pubBlock)

	return privKey, pubKey, nil
}

func EncryptRSA(pubKeyPEM []byte, plaintext string) (string, error) {
	block, _ := pem.Decode(pubKeyPEM)
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the public key")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return "", err
	}
	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", errors.New("not an RSA public key")
	}
	ciphertext, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, rsaPub, []byte(plaintext), nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

func DecryptRSA(privKeyPEM []byte, ciphertextBase64 string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(ciphertextBase64)
	if err != nil {
		return "", err
	}
	block, _ := pem.Decode(privKeyPEM)
	if block == nil {
		return "", errors.New("failed to parse PEM block containing the private key")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}
	plaintext, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, data, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

func GenerateDeviceAuthFields() (string, string, int64, string, error) {
	// 生成公私钥对（示例为 ECDSA）
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", 0, "", err
	}

	publicKey := privateKey.PublicKey
	// 生成签名数据
	nonce := fmt.Sprintf("%d", time.Now().UnixNano())
	signedAt := time.Now().Unix()

	dataToSign := fmt.Sprintf("%s%d", nonce, signedAt)
	hash := sha256.Sum256([]byte(dataToSign))

	// 使用私钥对数据进行签名
	signature, err := ecdsa.SignASN1(rand.Reader, privateKey, hash[:])
	if err != nil {
		return "", "", 0, "", err
	}

	// 返回生成的设备字段
	publicKeyStr := fmt.Sprintf("%x", publicKey.X) // 公钥（示例为 X 坐标）
	signatureStr := fmt.Sprintf("%x", signature)   // 签名（十六进制字符串）
	return publicKeyStr, signatureStr, signedAt, nonce, nil
}
