/*
Copyright (C) BABEC. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package paillier

import (
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strconv"

	"chainmaker.org/chainmaker-go/tools/cmc/util"
	"chainmaker.org/chainmaker/common/v2/crypto/paillier"
	"github.com/spf13/cobra"
)

var (
	// genKeyCmd flag vars
	paillierKeySavePath string
	paillierKeyFileName string

	paillierPrivKeyFilePath string
	paillierPubKeyFilePath  string
	paillierData            string
)

// PaillierCMD new ChainMaker paillier command
func PaillierCMD() *cobra.Command {
	paillierCmd := &cobra.Command{
		Use:   "paillier",
		Short: "ChainMaker paillier command",
		Long:  "ChainMaker paillier command",
	}

	paillierCmd.AddCommand(genKeyCMD())
	paillierCmd.AddCommand(encryptCMD())
	paillierCmd.AddCommand(decryptCMD())
	return paillierCmd
}

// genKeyCMD "generates paillier private public key
// @return *cobra.Command
func genKeyCMD() *cobra.Command {
	genKeyCmd := &cobra.Command{
		Use:   "genKey",
		Short: "generates paillier private public key",
		Long:  "generates paillier private public key",
		RunE: func(_ *cobra.Command, _ []string) error {
			return genKey()
		},
	}

	flags := genKeyCmd.Flags()
	flags.StringVarP(&paillierKeySavePath, "path", "", "",
		"the result storage file path, and the file name is the id")
	flags.StringVarP(&paillierKeyFileName, "name", "", "", "")

	return genKeyCmd
}

func genKey() error {
	prvFilePath := filepath.Join(paillierKeySavePath, fmt.Sprintf("%s.prvKey", paillierKeyFileName))
	pubFilePath := filepath.Join(paillierKeySavePath, fmt.Sprintf("%s.pubKey", paillierKeyFileName))

	_, err := pathExists(prvFilePath)
	if err != nil {
		return err
	}
	exist, err := pathExists(pubFilePath)
	if exist {
		return fmt.Errorf("file [ %s ] already exist", pubFilePath)
	}

	if err != nil {
		return err
	}
	prvKey, err := paillier.GenKey()
	if err != nil {
		return err
	}

	if exist {
		return fmt.Errorf("file [ %s ] already exist", prvFilePath)
	}
	pubKey, err := prvKey.GetPubKey()
	if err != nil {
		return err
	}

	pubKeyBytes, err := pubKey.Marshal()
	if err != nil {
		return err
	}

	prvKeyBytes, err := prvKey.Marshal()
	if err != nil {
		return err
	}

	if err = os.MkdirAll(paillierKeySavePath, os.ModePerm); err != nil {
		return fmt.Errorf("mk pailier dir failed, %s", err.Error())
	}

	if err = ioutil.WriteFile(prvFilePath,
		prvKeyBytes, 0600); err != nil {
		return fmt.Errorf("save paillier to file [%s] failed, %s", prvFilePath, err.Error())
	}
	fmt.Printf("[paillier Private Key] storage file path: %s\n", prvFilePath)

	if err = ioutil.WriteFile(pubFilePath,
		pubKeyBytes, 0600); err != nil {
		return fmt.Errorf("save paillier to file [%s] failed, %s", pubFilePath, err.Error())
	}
	fmt.Printf("[paillier Public Key] storage file path: %s\n", pubFilePath)
	return nil
}

// pathExists is used to determine whether a file or folder exists
func pathExists(path string) (bool, error) {
	if path == "" {
		return false, errors.New("invalid parameter, the file path cannot be empty")
	}
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// encryptCMD encrypt user data for paillier
// @return *cobra.Command
func encryptCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "encrypt",
		Short: "use paillier public key encrypt user data, the output encrypted data is in base64 format",
		Long:  "use paillier public key encrypt user data, the output encrypted data is in base64 format",
		RunE: func(_ *cobra.Command, _ []string) error {
			pubKeyBytes, err := ioutil.ReadFile(paillierPubKeyFilePath)
			if err != nil {
				return err
			}

			pubKey := new(paillier.PubKey)
			err = pubKey.Unmarshal(pubKeyBytes)
			if err != nil {
				return err
			}

			inData, err := strconv.ParseInt(paillierData, 10, 64)
			if err != nil {
				return err
			}
			cipherData, err := pubKey.Encrypt(new(big.Int).SetInt64(inData))
			if err != nil {
				return err
			}
			cipherBytes, err := cipherData.Marshal()
			if err != nil {
				return err
			}
			cipherStr := base64.StdEncoding.EncodeToString(cipherBytes)

			util.PrintPrettyJson(map[string]interface{}{
				"result": cipherStr,
			})
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&paillierPubKeyFilePath, "pubkey-file-path", "",
		"specify paillier public key file path")
	flags.StringVar(&paillierData, "data", "",
		"specify input data for encrypt. e.g. --data=\"123\"")

	return cmd
}

// decryptCMD decrypt user data for paillier
// @return *cobra.Command
func decryptCMD() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "decrypt",
		Short: "use paillier private key decrypt user data, the input encrypted data must in base64 format",
		Long:  "use paillier private key decrypt user data, the input encrypted data must in base64 format",
		RunE: func(_ *cobra.Command, _ []string) error {
			prvKeyBytes, err := ioutil.ReadFile(paillierPrivKeyFilePath)
			if err != nil {
				return err
			}
			prvKey := new(paillier.PrvKey)
			err = prvKey.Unmarshal(prvKeyBytes)
			if err != nil {
				return err
			}

			ct := new(paillier.Ciphertext)
			inData, err := base64.StdEncoding.DecodeString(paillierData)
			if err != nil {
				return err
			}
			err = ct.Unmarshal(inData)
			if err != nil {
				return err
			}

			decrypt, err := prvKey.Decrypt(ct)
			if err != nil {
				return err
			}

			util.PrintPrettyJson(map[string]interface{}{
				"result": decrypt.Int64(),
			})
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&paillierPrivKeyFilePath, "privkey-file-path", "",
		"specify paillier private key file path")
	flags.StringVar(&paillierData, "data", "",
		"specify input data for decrypt. e.g. --data=\"some base64 string\"")

	return cmd
}
