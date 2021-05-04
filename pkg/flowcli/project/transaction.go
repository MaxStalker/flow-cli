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

package project

import (
	"context"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"strings"

	jsoncdc "github.com/onflow/cadence/encoding/json"

	"github.com/onflow/flow-go-sdk/templates"

	"github.com/onflow/flow-cli/pkg/flowcli"

	"github.com/onflow/cadence"
	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-cli/pkg/flowcli/util"
)

const (
	defaultGasLimit = 1000
)

// NewTransaction create new instance of transaction
func NewTransaction() *Transaction {
	return &Transaction{
		tx: flow.NewTransaction(),
	}
}

// NewTransactionFromPayload build transaction from payload
func NewTransactionFromPayload(filename string) (*Transaction, error) {
	partialTxHex, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read partial transaction from %s: %v", filename, err)
	}

	partialTxBytes, err := hex.DecodeString(string(partialTxHex))
	if err != nil {
		return nil, fmt.Errorf("failed to decode partial transaction from %s: %v", filename, err)
	}

	decodedTx, err := flow.DecodeTransaction(partialTxBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to decode transaction from %s: %v", filename, err)
	}

	tx := &Transaction{
		tx: decodedTx,
	}

	return tx, nil
}

// NewUpdateAccountContractTransaction update account contract
func NewUpdateAccountContractTransaction(signer *Account, name string, source string) (*Transaction, error) {
	contract := templates.Contract{
		Name:   name,
		Source: source,
	}

	return newTransactionFromTemplate(
		templates.UpdateAccountContract(signer.Address(), contract),
		signer,
	)
}

// NewAddAccountContractTransaction add new contract to the account
func NewAddAccountContractTransaction(
	signer *Account,
	name string,
	source string,
	args []cadence.Value,
) (*Transaction, error) {
	return addAccountContractWithArgs(signer, templates.Contract{
		Name:   name,
		Source: source,
	}, args)
}

// NewRemoveAccountContractTransaction creates new transaction to remove contract
func NewRemoveAccountContractTransaction(signer *Account, name string) (*Transaction, error) {
	return newTransactionFromTemplate(
		templates.RemoveAccountContract(signer.Address(), name),
		signer,
	)
}

func addAccountContractWithArgs(
	signer *Account,
	contract templates.Contract,
	args []cadence.Value,
) (*Transaction, error) {
	const addAccountContractTemplate = `
	transaction(name: String, code: String %s) {
		prepare(signer: AuthAccount) {
			signer.contracts.add(name: name, code: code.decodeHex() %s)
		}
	}`

	cadenceName := cadence.NewString(contract.Name)
	cadenceCode := cadence.NewString(contract.SourceHex())

	tx := flow.NewTransaction().
		AddRawArgument(jsoncdc.MustEncode(cadenceName)).
		AddRawArgument(jsoncdc.MustEncode(cadenceCode)).
		AddAuthorizer(signer.Address())

	for _, arg := range args {
		arg.Type().ID()
		tx.AddRawArgument(jsoncdc.MustEncode(arg))
	}

	txArgs, addArgs := "", ""
	for i, arg := range args {
		txArgs += fmt.Sprintf(",arg%d:%s", i, arg.Type().ID())
		addArgs += fmt.Sprintf(",arg%d", i)
	}

	script := fmt.Sprintf(addAccountContractTemplate, txArgs, addArgs)
	tx.SetScript([]byte(script))

	t := &Transaction{tx: tx}
	err := t.SetSigner(signer)
	if err != nil {
		return nil, err
	}
	t.SetPayer(signer.Address())

	return t, nil
}

// NewCreateAccountTransaction creates new transaction for account
func NewCreateAccountTransaction(
	signer *Account,
	keys []*flow.AccountKey,
	contractArgs []string,
) (*Transaction, error) {

	contracts := make([]templates.Contract, 0)
	for _, contract := range contractArgs {
		contractFlagContent := strings.SplitN(contract, ":", 2)
		if len(contractFlagContent) != 2 {
			return nil, fmt.Errorf("wrong format for contract. Correct format is name:path, but got: %s", contract)
		}

		contractSource, err := util.LoadFile(contractFlagContent[1])
		if err != nil {
			return nil, err
		}

		contracts = append(contracts, templates.Contract{
			Name:   contractFlagContent[0],
			Source: string(contractSource),
		})
	}

	return newTransactionFromTemplate(
		templates.CreateAccount(keys, contracts, signer.Address()),
		signer,
	)
}

func newTransactionFromTemplate(templateTx *flow.Transaction, signer *Account) (*Transaction, error) {
	tx := &Transaction{tx: templateTx}

	err := tx.SetSigner(signer)
	if err != nil {
		return nil, err
	}
	tx.SetPayer(signer.Address())

	return tx, nil
}

// Transaction builder of flow transactions
type Transaction struct {
	signer   *Account
	proposer *flow.Account
	tx       *flow.Transaction
}

// Signer get signer
func (t *Transaction) Signer() *Account {
	return t.signer
}

// Proposer get proposer
func (t *Transaction) Proposer() *flow.Account {
	return t.proposer
}

// FlowTransaction get flow transaction
func (t *Transaction) FlowTransaction() *flow.Transaction {
	return t.tx
}

func (t *Transaction) SetScriptWithArgs(script []byte, args []string, argsJSON string) error {
	t.tx.SetScript(script)
	return t.AddRawArguments(args, argsJSON)
}

// SetSigner sets the signer for transaction
func (t *Transaction) SetSigner(account *Account) error {
	err := account.DefaultKey().Validate()
	if err != nil {
		return err
	}

	t.signer = account
	return nil
}

// SetProposer sets the proposer for transaction
func (t *Transaction) SetProposer(proposer *flow.Account, keyIndex int) *Transaction {
	t.proposer = proposer
	proposerKey := proposer.Keys[keyIndex]

	t.tx.SetProposalKey(
		proposer.Address,
		proposerKey.Index,
		proposerKey.SequenceNumber,
	)

	return t
}

// SetPayer sets the payer for transaction
func (t *Transaction) SetPayer(address flow.Address) *Transaction {
	t.tx.SetPayer(address)
	return t
}

// SetBlockReference sets block reference for transaction
func (t *Transaction) SetBlockReference(block *flow.Block) *Transaction {
	t.tx.SetReferenceBlockID(block.ID)
	return t
}

// SetDefaultGasLimit set the default gas limit for transaction
func (t *Transaction) SetDefaultGasLimit() *Transaction {
	t.tx.SetGasLimit(defaultGasLimit)
	return t
}

// AddRawArguments add raw arguments to tx
func (t *Transaction) AddRawArguments(args []string, argsJSON string) error {
	txArguments, err := flowcli.ParseArguments(args, argsJSON)
	if err != nil {
		return err
	}

	return t.AddArguments(txArguments)
}

// AddArguments add array of cadence arguments
func (t *Transaction) AddArguments(args []cadence.Value) error {
	for _, arg := range args {
		err := t.AddArgument(arg)
		if err != nil {
			return err
		}
	}

	return nil
}

// AddArgument add cadence typed argument
func (t *Transaction) AddArgument(arg cadence.Value) error {
	return t.tx.AddArgument(arg)
}

// AddAuthorizers add group of authorizers
func (t *Transaction) AddAuthorizers(authorizers []flow.Address) *Transaction {
	for _, authorizer := range authorizers {
		t.tx.AddAuthorizer(authorizer)
	}

	return t
}

// Sign signs transaction using signer account
func (t *Transaction) Sign() (*Transaction, error) {
	keyIndex := t.signer.DefaultKey().Index()
	signer, err := t.signer.DefaultKey().Signer(context.Background())
	if err != nil {
		return nil, err
	}

	if t.shouldSignEnvelope() {
		err = t.tx.SignEnvelope(t.signer.address, keyIndex, signer)
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %s", err)
		}
	} else {
		err = t.tx.SignPayload(t.signer.address, keyIndex, signer)
		if err != nil {
			return nil, fmt.Errorf("failed to sign transaction: %s", err)
		}
	}

	return t, nil
}

// shouldSignEnvelope checks if signer should sign envelope or payload
func (t *Transaction) shouldSignEnvelope() bool {
	// if signer is payer
	return t.signer.address == t.tx.Payer // todo discuss
}