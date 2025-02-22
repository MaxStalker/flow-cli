/*
 * Flow CLI
 *
 * Copyright 2019 Dapper Labs, Inc.
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
	"bytes"
	"fmt"
	"strings"

	"github.com/onflow/flow-cli/pkg/flowkit"
	"github.com/onflow/flow-go-sdk"

	"github.com/onflow/flow-cli/pkg/flowkit/config"

	"github.com/onflow/flow-go-sdk/crypto"

	"github.com/onflow/flow-cli/pkg/flowkit/contracts"
	"github.com/onflow/flow-cli/pkg/flowkit/gateway"
	"github.com/onflow/flow-cli/pkg/flowkit/output"
)

// Project is a service that handles all interactions for a state.
type Project struct {
	gateway gateway.Gateway
	state   *flowkit.State
	logger  output.Logger
}

// NewProject returns a new state service.
func NewProject(
	gateway gateway.Gateway,
	state *flowkit.State,
	logger output.Logger,
) *Project {
	return &Project{
		gateway: gateway,
		state:   state,
		logger:  logger,
	}
}

// Init initializes a new project using the properties provided.
func (p *Project) Init(
	readerWriter flowkit.ReaderWriter,
	reset bool,
	global bool,
	sigAlgo crypto.SignatureAlgorithm,
	hashAlgo crypto.HashAlgorithm,
	serviceKey crypto.PrivateKey,
) (*flowkit.State, error) {
	path := config.DefaultPath
	if global {
		path = config.GlobalPath()
	}

	if flowkit.Exists(path) && !reset {
		return nil, fmt.Errorf(
			"configuration already exists at: %s, if you want to reset configuration use the reset flag",
			path,
		)
	}

	state, err := flowkit.Init(readerWriter, sigAlgo, hashAlgo)
	if err != nil {
		return nil, err
	}

	if serviceKey != nil {
		state.SetEmulatorKey(serviceKey)
	}

	err = state.Save(path)
	if err != nil {
		return nil, err
	}

	return state, nil
}

// Defines a Mainnet Standard Contract ( e.g Core Contracts, FungibleToken, NonFungibleToken )
type StandardContract struct {
	Name     string
	Address  flow.Address
	InfoLink string
}

func (p *Project) ReplaceStandardContractReferenceToAlias(standardContract StandardContract) error {
	//replace contract with alias
	c, err := p.state.Config().Contracts.ByNameAndNetwork(standardContract.Name, config.DefaultMainnetNetwork().Name)
	if err != nil {
		return err
	}
	c.Alias = standardContract.Address.String()

	//remove from deploy
	for di, d := range p.state.Config().Deployments {
		if d.Network != config.DefaultMainnetNetwork().Name {
			continue
		}
		for ci, c := range d.Contracts {
			if c.Name == standardContract.Name {
				p.state.Config().Deployments[di].Contracts = append((d.Contracts)[0:ci], (d.Contracts)[ci+1:]...)
				break
			}
		}
	}
	return nil
}

func (p *Project) CheckForStandardContractUsageOnMainnet() error {

	mainnetContracts := map[string]StandardContract{
		"FungibleToken": {
			Name:     "FungibleToken",
			Address:  flow.HexToAddress("0xf233dcee88fe0abe"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/fungible-token",
		},
		"FlowToken": {
			Name:     "FlowToken",
			Address:  flow.HexToAddress("0x1654653399040a61"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/flow-token",
		},
		"FlowFees": {
			Name:     "FlowFees",
			Address:  flow.HexToAddress("0xf919ee77447b7497"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/flow-fees",
		},
		"FlowServiceAccount": {
			Name:     "FlowServiceAccount",
			Address:  flow.HexToAddress("0xe467b9dd11fa00df"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/service-account",
		},
		"FlowStorageFees": {
			Name:     "FlowStorageFees",
			Address:  flow.HexToAddress("0xe467b9dd11fa00df"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/service-account",
		},
		"FlowIDTableStaking": {
			Name:     "FlowIDTableStaking",
			Address:  flow.HexToAddress("0x8624b52f9ddcd04a"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/staking-contract-reference",
		},
		"FlowEpoch": {
			Name:     "FlowEpoch",
			Address:  flow.HexToAddress("0x8624b52f9ddcd04a"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/epoch-contract-reference",
		},
		"FlowClusterQC": {
			Name:     "FlowClusterQC",
			Address:  flow.HexToAddress("0x8624b52f9ddcd04a"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/epoch-contract-reference",
		},
		"FlowDKG": {
			Name:     "FlowDKG",
			Address:  flow.HexToAddress("0x8624b52f9ddcd04a"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/epoch-contract-reference",
		},
		"NonFungibleToken": {
			Name:     "NonFungibleToken",
			Address:  flow.HexToAddress("0x1d7e57aa55817448"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/non-fungible-token",
		},
		"MetadataViews": {
			Name:     "MetadataViews",
			Address:  flow.HexToAddress("0x1d7e57aa55817448"),
			InfoLink: "https://developers.flow.com/flow/core-contracts/nft-metadata",
		},
	}

	contracts, err := p.state.DeploymentContractsByNetwork("mainnet")
	if err != nil {
		return err
	}

	for _, contract := range contracts {
		standardContract, ok := mainnetContracts[contract.Name]
		if !ok {
			continue
		}

		p.logger.Info(fmt.Sprintf("It seems like you are trying to deploy %s to Mainnet \n", contract.Name))
		p.logger.Info(fmt.Sprintf("It is a standard contract already deployed at address 0x%s \n", standardContract.Address.String()))
		p.logger.Info(fmt.Sprintf("You can read more about it here: %s \n", standardContract.InfoLink))

		if output.WantToUseMainnetVersionPrompt() {
			err := p.ReplaceStandardContractReferenceToAlias(standardContract)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Deploy the project for the provided network.
//
// Retrieve all the contracts for specified network, sort them for deployment
// deploy one by one and replace the imports in the contract source so it corresponds
// to the account name the contract was deployed to.
func (p *Project) Deploy(network string, update bool) ([]*contracts.Contract, error) {
	if p.state == nil {
		return nil, config.ErrDoesNotExist
	}
	// check there are not multiple accounts with same contract
	if p.state.ContractConflictExists(network) {
		return nil, fmt.Errorf( // TODO(sideninja) specify which contract by name is a problem
			"the same contract cannot be deployed to multiple accounts on the same network",
		)
	}

	// create new processor for contract
	processor := contracts.NewPreprocessor(
		contracts.FilesystemLoader{
			Reader: p.state.ReaderWriter(),
		},
		p.state.AliasesForNetwork(network),
	)

	// add all contracts needed to deploy to processor
	contractsNetwork, err := p.state.DeploymentContractsByNetwork(network)
	if err != nil {
		return nil, err
	}

	for _, contract := range contractsNetwork {
		err := processor.AddContractSource(
			contract.Name,
			contract.Source,
			contract.AccountAddress,
			contract.AccountName,
			contract.Args,
		)
		if err != nil {
			return nil, err
		}
	}

	// resolve imports assigns accounts to imports
	err = processor.ResolveImports()
	if err != nil {
		return nil, err
	}

	// sort correct deployment order of contracts so we don't have import that is not yet deployed
	orderedContracts, err := processor.ContractDeploymentOrder()
	if err != nil {
		return nil, err
	}

	p.logger.Info(fmt.Sprintf(
		"\nDeploying %d contracts for accounts: %s\n",
		len(orderedContracts),
		strings.Join(p.state.AccountNamesForNetwork(network), ","),
	))
	defer p.logger.StopProgress()

	deployErr := false
	numOfUpdates := 0
	for _, contract := range orderedContracts {
		block, err := p.gateway.GetLatestBlock()
		if err != nil {
			return nil, err
		}

		targetAccount, err := p.state.Accounts().ByName(contract.AccountName())

		if err != nil {
			return nil, fmt.Errorf("target account for deploying contract not found in configuration")
		}

		// get deployment account
		targetAccountInfo, err := p.gateway.GetAccount(targetAccount.Address())
		if err != nil {
			return nil, fmt.Errorf("failed to fetch information for account %s with error %s", targetAccount.Address(), err.Error())
		}

		// create transaction to deploy new contract with args
		tx, err := flowkit.NewAddAccountContractTransaction(
			targetAccount,
			contract.Name(),
			contract.TranspiledCode(),
			contract.Args(),
		)
		if err != nil {
			return nil, err
		}
		// check if contract exists on account
		existingContract, exists := targetAccountInfo.Contracts[contract.Name()]
		noDiffInContract := bytes.Equal([]byte(contract.TranspiledCode()), existingContract)

		if exists && !update {
			p.logger.Error(fmt.Sprintf(
				"contract %s is already deployed to this account. Use the --update flag to force update",
				contract.Name(),
			))
			deployErr = true
			continue
		} else if exists && len(contract.Args()) > 0 { // TODO(sideninja) discuss removing the contract and redeploying it
			p.logger.Error(fmt.Sprintf(
				"contract %s is already deployed and can not be updated with initialization arguments",
				contract.Name(),
			))
			deployErr = true
			continue
		} else if exists {
			//only update contract if there is diff
			if noDiffInContract {
				p.logger.Info(fmt.Sprintf(
					"no diff found in %s, skipping update",
					contract.Name(),
				))
				continue
			}
			tx, err = flowkit.NewUpdateAccountContractTransaction(targetAccount, contract.Name(), contract.TranspiledCode())
			if err != nil {
				return nil, err
			}
			numOfUpdates++
		}

		tx.SetBlockReference(block)

		if err = tx.SetProposer(targetAccountInfo, targetAccount.Key().Index()); err != nil {
			return nil, err
		}

		tx, err = tx.Sign()
		if err != nil {
			p.logger.Error(fmt.Sprintf("%s error: %s", contract.Name(), err))
			deployErr = true
			continue
		}

		p.logger.StartProgress(
			fmt.Sprintf("%s deploying...", output.Bold(contract.Name())),
		)

		sentTx, err := p.gateway.SendSignedTransaction(tx)
		if err != nil {
			p.logger.StopProgress()
			p.logger.Error(fmt.Sprintf("%s error: %s", contract.Name(), err))
			deployErr = true
			continue
		}

		result, err := p.gateway.GetTransactionResult(sentTx.ID(), true)
		if err != nil {
			p.logger.StopProgress()
			p.logger.Error(fmt.Sprintf("%s error: %s", contract.Name(), err))
			deployErr = true
			continue
		}
		if result == nil {
			p.logger.Error("could not fetch the result of deployment, skipping")
			deployErr = true
			continue
		}
		if result.Error != nil {
			deployErr = true
			p.logger.StopProgress()
			if exists && update {
				p.logger.Error(fmt.Sprintf(
					"Error updating %s: (%s)\n",
					output.Red(contract.Name()),
					result.Error.Error(),
				))
			} else {
				p.logger.Error(fmt.Sprintf(
					"Error deploying %s: (%s)\n",
					output.Red(contract.Name()),
					result.Error.Error(),
				))
			}
		}

		if result.Error == nil && !deployErr {
			changeStatus := ""
			if exists && update {
				changeStatus = "(update)"
			}
			p.logger.StopProgress()
			p.logger.Info(fmt.Sprintf(
				"%s -> 0x%s (%s) %s\n",
				output.Green(contract.Name()),
				contract.Target(),
				sentTx.ID().String(),
				changeStatus,
			))
		}
	}

	if !deployErr {
		if update && numOfUpdates > 0 {
			p.logger.Info(fmt.Sprintf("%d contracts updated successfully", numOfUpdates))
		}
		p.logger.Info(fmt.Sprintf("\n%s All contracts deployed successfully", output.SuccessEmoji()))
	} else {
		err = fmt.Errorf("failed to deploy all contracts")
		p.logger.Error(err.Error())
		return nil, err
	}

	return orderedContracts, nil
}
