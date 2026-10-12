package shardingsync

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"

	"github.com/libp2p/go-libp2p-core/crypto"
)

// loadPrivateKey 从文件加载私钥并转换为 libp2p 格式
// 支持标准库可解析的 RSA 和 ECDSA PEM 私钥。
func loadPrivateKey(keyPath string) (crypto.PrivKey, error) {
	keyBytes, err := ioutil.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("读取私钥文件失败: %v", err)
	}

	block, _ := pem.Decode(keyBytes)
	if block == nil {
		return nil, fmt.Errorf("解码 PEM 失败")
	}

	// 1. 尝试解析 PKCS8
	if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
		return convertToLibp2pKey(key)
	}

	// 2. 尝试解析 EC 私钥 (SEC1)
	if key, err := x509.ParseECPrivateKey(block.Bytes); err == nil {
		return convertToLibp2pKey(key)
	}

	// 3. 尝试解析 RSA 私钥 (PKCS1)
	if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return convertToLibp2pKey(key)
	}

	return nil, fmt.Errorf("无法识别的私钥格式")
}

// convertToLibp2pKey 将标准库私钥转换为 libp2p 私钥
func convertToLibp2pKey(privKey interface{}) (crypto.PrivKey, error) {
	switch k := privKey.(type) {
	case *rsa.PrivateKey:
		// 将 RSA 私钥转换为字节流
		b := x509.MarshalPKCS1PrivateKey(k)
		return crypto.UnmarshalRsaPrivateKey(b)
	case *ecdsa.PrivateKey:
		// UnmarshalECDSAPrivateKey 接收 SEC1 DER 编码。
		b, err := x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}

		return crypto.UnmarshalECDSAPrivateKey(b)
	default:
		return nil, fmt.Errorf("不支持的私钥类型: %T", privKey)
	}
}
