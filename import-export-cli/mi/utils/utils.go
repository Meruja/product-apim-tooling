/*
*  Copyright (c) WSO2 Inc. (http://www.wso2.org) All Rights Reserved.
*
*  WSO2 Inc. licenses this file to you under the Apache License,
*  Version 2.0 (the "License"); you may not use this file except
*  in compliance with the License.
*  You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing,
* software distributed under the License is distributed on an
* "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
* KIND, either express or implied.  See the License for the
* specific language governing permissions and limitations
* under the License.
 */

package utils

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/x509"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/magiconair/properties"
	"github.com/pavel-v-chernykh/keystore-go/v4"
	"github.com/wso2/product-apim-tooling/import-export-cli/utils"
)

type k8sSecretConfig struct {
	APIVerion  string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	MetaData   metaData          `yaml:"metadata"`
	StringData map[string]string `yaml:"stringData"`
	Type       string            `yaml:"type"`
}

type metaData struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
}

type SecretConfig struct {
	OutputType          string
	Algorithm           string
	InputType           string
	InputFile           string
	PlainTextAlias      string
	PlainTextSecretText string
}

type encryptFunc func(key *rsa.PublicKey, plainText string) (string, error)

// GetTrimmedCmdLiteral returns the command without the arguments
func GetTrimmedCmdLiteral(cmd string) string {
	cmdParts := strings.Fields(cmd)
	return cmdParts[0]
}

// IsMapWithNonEmptyValues iterates over a map and return false if there is an empty value
func IsMapWithNonEmptyValues(inputs map[string]string) bool {
	for key, input := range inputs {
		if len(strings.TrimSpace(input)) == 0 {
			fmt.Println("Invalid input for " + key)
			return false
		}
	}
	return true
}

// EncryptSecrets encrypts the secrets using the keystore and write them to a file or console depending on the config map argument
func EncryptSecrets(keyStorePropertiesFile string, secretConfig SecretConfig) error {
	keyStoreConfigMap := readPropertiesFromFile(keyStorePropertiesFile)
	encryptionKey, err := getEncryptionKey(keyStoreConfigMap)
	if err != nil {
		return err
	}
	var encryptedSecrets map[string]string
	plainTextSecrets := getPlainTextSecrets(secretConfig)

	if IsPKCS1Encryption(secretConfig.Algorithm) {
		encryptedSecrets, err = encrypt(encryptionKey, plainTextSecrets, encryptPKCS1v15)
	} else {
		encryptedSecrets, err = encrypt(encryptionKey, plainTextSecrets, encryptOAEP)
	}
	if err != nil {
		return err
	}
	if IsK8(secretConfig.OutputType) {
		printSecretsToYamlFile(encryptedSecrets)
	} else if IsFile(secretConfig.OutputType) {
		printSecretsToPropertiesFile(encryptedSecrets)
	} else {
		printSecretsToConsole(encryptedSecrets)
	}
	return nil
}

// WritePropertiesToFile write a map to a .properties file
func WritePropertiesToFile(variables map[string]string, fileName string) {
	props := properties.LoadMap(variables)
	writer, err := os.Create(fileName)
	if err != nil {
		utils.HandleErrorAndExit("Unable to create file.", err)
	}
	_, err = props.Write(writer, properties.UTF8)
	if err != nil {
		utils.HandleErrorAndExit("Unable to write properties to file.", err)
	}
	writer.Close()
}

func readPropertiesFromFile(fileName string) map[string]string {
	props := properties.MustLoadFile(fileName, properties.UTF8)
	return props.Map()
}

// GetSecurityDirectoryPath join mi-security with the config directory path
func GetSecurityDirectoryPath() string {
	return filepath.Join(utils.ConfigDirPath, "mi-security")
}

// GetkeyStorePropertiesFilePath join keystore-info.properties with the mi-security path
func GetkeyStorePropertiesFilePath() string {
	return filepath.Join(GetSecurityDirectoryPath(), "keystore-info.properties")
}

func getEncryptionKey(keyStoreConfigMap map[string]string) (*rsa.PublicKey, error) {
	keyStorePath := keyStoreConfigMap["secret.keystore.location"]
	keyStorePassword, _ := base64.StdEncoding.DecodeString(keyStoreConfigMap["secret.keystore.password"])
	keyStore, err := readKeyStore(keyStorePath, keyStorePassword)
	if err != nil {
		return nil, errors.New("Reading Key Store: " + err.Error())
	}
	keyAlias := keyStoreConfigMap["secret.keystore.key.alias"]
	keyPassword, _ := base64.StdEncoding.DecodeString(keyStoreConfigMap["secret.keystore.key.password"])
	pke, err := keyStore.GetPrivateKeyEntry(keyAlias, keyPassword)
	if err != nil {
		return nil, errors.New("Reading Key Entry: " + err.Error())
	}
	key, err := x509.ParsePKCS8PrivateKey(pke.PrivateKey)
	rsaKey := key.(*rsa.PrivateKey)
	if err != nil {
		return nil, errors.New("Parsing Key Entry: " + err.Error())
	}
	return &rsaKey.PublicKey, nil
}

func encrypt(encryptionKey *rsa.PublicKey, plainTextSecrets map[string]string, encryptFunction encryptFunc) (map[string]string, error) {
	var encryptedSecrets = make(map[string]string)
	for alias, plainText := range plainTextSecrets {
		encryptedSecret, err := encryptFunction(encryptionKey, plainText)
		if err != nil {
			return nil, err
		}
		encryptedSecrets[alias] = encryptedSecret
	}
	return encryptedSecrets, nil
}

func getPlainTextSecrets(secretConfig SecretConfig) map[string]string {
	var plainTexts = make(map[string]string)
	if IsFile(secretConfig.InputType) {
		plainTexts = readPropertiesFromFile(secretConfig.InputFile)
	} else {
		plainTexts[secretConfig.PlainTextAlias] = secretConfig.PlainTextSecretText
	}
	return plainTexts
}

func printSecretsToConsole(secrets map[string]string) {
	for alias, secret := range secrets {
		fmt.Println(alias, ":", secret)
	}
}

func printSecretsToPropertiesFile(secrets map[string]string) {
	secretFilePath := getSecretFilePath("wso2mi-secrets.properties")
	WritePropertiesToFile(secrets, secretFilePath)
	fmt.Println("Secret properties file created in", secretFilePath)
}

func printSecretsToYamlFile(secrets map[string]string) {
	secretConfig := k8sSecretConfig{
		APIVerion:  "v1",
		Kind:       "Secret",
		StringData: secrets,
		Type:       "Opaque",
		MetaData: metaData{
			Name:      "wso2misecret",
			Namespace: "default",
		},
	}
	secretFilePath := getSecretFilePath("wso2mi-secrets.yaml")
	utils.WriteConfigFile(secretConfig, secretFilePath)
	fmt.Println("Kubernetes secret file created in", secretFilePath, "with default name and namespace")
	fmt.Println("You can change the default values as required before applying.")
}

func getSecretFilePath(fileName string) string {
	currentDir, _ := os.Getwd()
	secretDirPath := filepath.Join(currentDir, "security")
	utils.CreateDirIfNotExist(secretDirPath)
	return filepath.Join(secretDirPath, fileName)
}

func encryptOAEP(key *rsa.PublicKey, plainText string) (string, error) {
	encryptedBytes, err := rsa.EncryptOAEP(sha1.New(), rand.Reader, key, []byte(plainText), nil)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encryptedBytes), nil
}

func encryptPKCS1v15(key *rsa.PublicKey, plainText string) (string, error) {
	encryptedBytes, err := rsa.EncryptPKCS1v15(rand.Reader, key, []byte(plainText))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(encryptedBytes), nil
}

func readKeyStore(filename string, password []byte) (*keystore.KeyStore, error) {
	f, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer func() {
		f.Close()
	}()
	keyStore := keystore.New()
	if err := keyStore.Load(f, password); err != nil {
		return nil, err
	}
	return &keyStore, nil
}

// IsConsole return true if outputType is console
func IsConsole(outputType string) bool {
	return strings.EqualFold(outputType, "console")
}

// IsFile return true if outputType is file
func IsFile(outputType string) bool {
	return strings.EqualFold(outputType, "file")
}

// IsK8 return true if outputType is k8
func IsK8(outputType string) bool {
	return strings.EqualFold(outputType, "k8")
}

// IsPKCS1Encryption return true if the encryption algorithm is RSA/ECB/PKCS1Padding
func IsPKCS1Encryption(algorithm string) bool {
	return strings.EqualFold(algorithm, "RSA/ECB/PKCS1Padding")
}

// IsOAEPEncryption return true if the encryption algorithm is RSA/ECB/OAEPWithSHA1AndMGF1Padding
func IsOAEPEncryption(algorithm string) bool {
	return strings.EqualFold(algorithm, "RSA/ECB/OAEPWithSHA1AndMGF1Padding")
}
