/*
 * Copyright (C) 2018 The dad-go Authors
 * This file is part of The dad-go library.
 *
 * The dad-go is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The dad-go is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with The dad-go.  If not, see <http://www.gnu.org/licenses/>.
 */

package genesis

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/ontio/dad-go-crypto/keypair"
	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/common/config"
	"github.com/ontio/dad-go/common/constants"
	"github.com/ontio/dad-go/consensus/vbft/config"
	"github.com/ontio/dad-go/core/types"
	"github.com/ontio/dad-go/core/utils"
	"github.com/ontio/dad-go/smartcontract/service/native/global_params"
	"github.com/ontio/dad-go/smartcontract/service/native/governance"
	"github.com/ontio/dad-go/smartcontract/service/native/ont"
	nutils "github.com/ontio/dad-go/smartcontract/service/native/utils"
	"github.com/ontio/dad-go/smartcontract/service/neovm"
)

const (
	BlockVersion uint32 = 0
	GenesisNonce uint64 = 2083236893
)

var (
	ONTToken   = newGoverningToken()
	ONGToken   = newUtilityToken()
	ONTTokenID = ONTToken.Hash()
	ONGTokenID = ONGToken.Hash()
)

var GenBlockTime = (config.DEFAULT_GEN_BLOCK_TIME * time.Second)

var INIT_PARAM = map[string]string{
	"gasPrice": "0",
}

var GenesisBookkeepers []keypair.PublicKey

// BuildGenesisBlock returns the genesis block with default consensus bookkeeper list
func BuildGenesisBlock(defaultBookkeeper []keypair.PublicKey, genesisConfig *config.GenesisConfig) (*types.Block, error) {
	//getBookkeeper
	GenesisBookkeepers = defaultBookkeeper
	nextBookkeeper, err := types.AddressFromBookkeepers(defaultBookkeeper)
	if err != nil {
		return nil, fmt.Errorf("[Block],BuildGenesisBlock err with GetBookkeeperAddress: %s", err)
	}
	conf := bytes.NewBuffer(nil)
	if genesisConfig.VBFT != nil {
		genesisConfig.VBFT.Serialize(conf)
	}
	govConfig := newGoverConfigInit(conf.Bytes())
	consensusPayload, err := vconfig.GenesisConsensusPayload(govConfig.Hash(), 0)
	if err != nil {
		return nil, fmt.Errorf("consensus genesus init failed: %s", err)
	}
	//blockdata
	genesisHeader := &types.Header{
		Version:          BlockVersion,
		PrevBlockHash:    common.Uint256{},
		TransactionsRoot: common.Uint256{},
		Timestamp:        constants.GENESIS_BLOCK_TIMESTAMP,
		Height:           uint32(0),
		ConsensusData:    GenesisNonce,
		NextBookkeeper:   nextBookkeeper,
		ConsensusPayload: consensusPayload,

		Bookkeepers: nil,
		SigData:     nil,
	}

	//block
	ont := newGoverningToken()
	ong := newUtilityToken()
	param := newParamContract()
	oid := deployOntIDContract()
	auth := deployAuthContract()
	config := newConfig()

	genesisBlock := &types.Block{
		Header: genesisHeader,
		Transactions: []*types.Transaction{
			ont,
			ong,
			param,
			oid,
			auth,
			config,
			newGoverningInit(),
			newUtilityInit(),
			newParamInit(),
			govConfig,
		},
	}
	genesisBlock.RebuildMerkleRoot()
	return genesisBlock, nil
}

func newGoverningToken() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.OntContractAddress[:], "ONT", "1.0",
		"dad-go Team", "contact@ont.io", "dad-go Network ONT Token", true)
	return tx
}

func newUtilityToken() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.OngContractAddress[:], "ONG", "1.0",
		"dad-go Team", "contact@ont.io", "dad-go Network ONG Token", true)
	return tx
}

func newParamContract() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.ParamContractAddress[:],
		"ParamConfig", "1.0", "dad-go Team", "contact@ont.io",
		"Chain Global Environment Variables Manager ", true)
	return tx
}

func newConfig() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.GovernanceContractAddress[:], "CONFIG", "1.0",
		"dad-go Team", "contact@ont.io", "dad-go Network Consensus Config", true)
	return tx
}

func deployAuthContract() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.AuthContractAddress[:], "AuthContract", "1.0",
		"dad-go Team", "contact@ont.io", "dad-go Network Authorization Contract", true)
	return tx
}

func deployOntIDContract() *types.Transaction {
	tx := utils.NewDeployTransaction(nutils.OntIDContractAddress[:], "OID", "1.0",
		"dad-go Team", "contact@ont.io", "dad-go Network ONT ID", true)
	return tx
}

func newGoverningInit() *types.Transaction {
	bookkeepers, _ := config.DefConfig.GetBookkeepers()
	addr := types.AddressFromPubKey(bookkeepers[0])

	distribute := []struct {
		addr  common.Address
		value uint64
	}{{addr, constants.ONT_TOTAL_SUPPLY}}

	args := bytes.NewBuffer(nil)
	nutils.WriteVarUint(args, uint64(len(distribute)))
	for _, part := range distribute {
		nutils.WriteAddress(args, part.addr)
		nutils.WriteVarUint(args, part.value)
	}

	return utils.Buildad-gotiveTransaction(nutils.OntContractAddress, ont.INIT_NAME, args.Bytes())
}

func newUtilityInit() *types.Transaction {
	return utils.Buildad-gotiveTransaction(nutils.OngContractAddress, ont.INIT_NAME, []byte{})
}

func newParamInit() *types.Transaction {
	params := new(global_params.Params)
	for k, v := range INIT_PARAM {
		params.SetParam(global_params.Param{k, v})
	}
	for k, v := range neovm.GAS_TABLE {
		params.SetParam(global_params.Param{k, strconv.FormatUint(v, 10)})
	}
	bf := new(bytes.Buffer)
	params.Serialize(bf)

	bookkeepers, _ := config.DefConfig.GetBookkeepers()
	address := types.AddressFromPubKey(bookkeepers[0])
	nutils.WriteAddress(bf, address)
	return utils.Buildad-gotiveTransaction(nutils.ParamContractAddress, global_params.INIT_NAME, bf.Bytes())
}

func newGoverConfigInit(config []byte) *types.Transaction {
	return utils.Buildad-gotiveTransaction(nutils.GovernanceContractAddress, governance.INIT_CONFIG, config)
}
