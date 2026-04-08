package aibot

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
)

// DecryptFile 使用 AES-256-CBC 解密文件
// encryptedData: 加密的文件数据
// aesKey: Base64 编码的 AES-256 密钥
// 返回：解密后的文件数据
func DecryptFile(encryptedData []byte, aesKey string) ([]byte, error) {
	if len(encryptedData) == 0 {
		return nil, errors.New("decrypt_file: encrypted_data is empty or not provided")
	}

	if aesKey == "" {
		return nil, errors.New("decrypt_file: aes_key must be a non-empty string")
	}

	// 将 Base64 编码的 aesKey 解码为 bytes
	// Go 的 base64.StdEncoding.DecodeString 会自动处理缺少的 '=' padding
	key, err := base64.StdEncoding.DecodeString(aesKey)
	if err != nil {
		// 尝试添加 padding 后重新解码
		paddedAesKey := aesKey
		if len(aesKey)%4 != 0 {
			paddedAesKey = aesKey + strings.Repeat("=", 4-len(aesKey)%4)
		}
		key, err = base64.StdEncoding.DecodeString(paddedAesKey)
		if err != nil {
			return nil, fmt.Errorf("decrypt_file: failed to decode aes_key: %w", err)
		}
	}

	// IV 取 aesKey 解码后的前 16 字节
	iv := key[:16]

	// 创建 AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("decrypt_file: failed to create AES cipher: %w", err)
	}

	// 创建 CBC 解密器
	mode := cipher.NewCBCDecrypter(block, iv)

	// 确保加密数据长度是 AES block size (16 字节) 的倍数
	blockSize := 16
	remainder := len(encryptedData) % blockSize
	if remainder != 0 {
		// 补零对齐
		padding := blockSize - remainder
		encryptedData = append(encryptedData, make([]byte, padding)...)
	}

	// 解密
	decrypted := make([]byte, len(encryptedData))
	mode.CryptBlocks(decrypted, encryptedData)

	// 去除 PKCS#7 填充
	if len(decrypted) == 0 {
		return nil, errors.New("Decrypted data is empty")
	}

	padLen := int(decrypted[len(decrypted)-1])
	if padLen < 1 || padLen > 32 || padLen > len(decrypted) {
		return nil, fmt.Errorf("Invalid PKCS#7 padding value: %d", padLen)
	}

	// 验证所有 padding 字节是否一致
	for i := len(decrypted) - padLen; i < len(decrypted); i++ {
		if decrypted[i] != byte(padLen) {
			return nil, errors.New("Invalid PKCS#7 padding: padding bytes mismatch")
		}
	}

	return decrypted[:len(decrypted)-padLen], nil
}
