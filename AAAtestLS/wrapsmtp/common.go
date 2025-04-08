package main

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
)

var AESKey []byte

func LoadConfig() {
	data, _ := ioutil.ReadFile("config.json")
	var config map[string]string
	json.Unmarshal(data, &config)
	AESKey = []byte(config["aes_key"])
}

func AESEncrypt(data []byte) []byte {
	block, _ := aes.NewCipher(AESKey)
	iv := AESKey[:aes.BlockSize]
	mode := cipher.NewCBCEncrypter(block, iv)
	padding := aes.BlockSize - len(data)%aes.BlockSize
	for i := 0; i < padding; i++ {
		data = append(data, byte(padding))
	}
	encrypted := make([]byte, len(data))
	mode.CryptBlocks(encrypted, data)
	return encrypted
}

func AESDecrypt(data []byte) []byte {
	block, _ := aes.NewCipher(AESKey)
	iv := AESKey[:aes.BlockSize]
	mode := cipher.NewCBCDecrypter(block, iv)
	decrypted := make([]byte, len(data))
	mode.CryptBlocks(decrypted, data)
	length := len(decrypted) - int(decrypted[len(decrypted)-1])
	return decrypted[:length]
}

func Encode(data []byte) []byte {
	return []byte(base64.StdEncoding.EncodeToString(data))
}

func Decode(data []byte) []byte {
	result, _ := base64.StdEncoding.DecodeString(string(data))
	return result
}
