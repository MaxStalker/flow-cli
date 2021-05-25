/*
 * Flow CLI
 *
 * Copyright 2019-2021 Dapper Labs, Inc.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package services

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/onflow/flow-go-sdk"
	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-cli/pkg/flowcli/gateway"
	"github.com/onflow/flow-cli/pkg/flowcli/output"
	"github.com/onflow/flow-cli/pkg/flowcli/project"
	"github.com/onflow/flow-cli/pkg/flowcli/util"
)

// Keys is a service that handles all key-related interactions.
type Keys struct {
	gateway gateway.Gateway
	project *project.Project
	logger  output.Logger
}

// NewKeys returns a new keys service.
func NewKeys(
	gateway gateway.Gateway,
	project *project.Project,
	logger output.Logger,
) *Keys {
	return &Keys{
		gateway: gateway,
		project: project,
		logger:  logger,
	}
}

const PEM string = "pem"
const RLP string = "rlp"

// Generate generates a new private key from the given seed and signature algorithm.
func (k *Keys) Generate(inputSeed string, signatureAlgo string) (crypto.PrivateKey, error) {
	var seed []byte
	var err error

	if inputSeed == "" {
		seed, err = util.RandomSeed(crypto.MinSeedLength)
		if err != nil {
			return nil, err
		}
	} else {
		seed = []byte(inputSeed)
	}

	sigAlgo := crypto.StringToSignatureAlgorithm(signatureAlgo)
	if sigAlgo == crypto.UnknownSignatureAlgorithm {
		return nil, fmt.Errorf("invalid signature algorithm: %s", signatureAlgo)
	}

	privateKey, err := crypto.GeneratePrivateKey(sigAlgo, seed)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %v", err)
	}

	return privateKey, nil
}

// Decode encoded public key for supported encoding type
func (k *Keys) Decode(encoded string, encoding string) (*flow.AccountKey, error) {
	if strings.ToLower(encoding) == PEM {
		return decodePEM(encoded)
	} else if strings.ToLower(encoding) == RLP {
		return decodeRLP(encoded)
	}

	return nil, fmt.Errorf("encoding type not supported. Valid encoding: RLP and PEM")
}

func decodeRLP(publicKey string) (*flow.AccountKey, error) {
	publicKeyBytes, err := hex.DecodeString(publicKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decode public key: %v", err)
	}

	accountKey, err := flow.DecodeAccountKey(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode: %v", err)
	}

	return accountKey, nil
}

func decodePEM(publicKeyPath string) (*flow.AccountKey, error) {
	fileContent, err := util.LoadFile(publicKeyPath)
	if err != nil {
		return nil, err
	}

	pk, err := crypto.DecodePublicKeyPEM(crypto.ECDSA_P256, string(fileContent))
	if err != nil {
		return nil, err
	}

	return &flow.AccountKey{
		PublicKey: pk,
		SigAlgo:   crypto.ECDSA_P256,
		HashAlgo:  crypto.SHA3_256, // refactor
	}, nil
}
