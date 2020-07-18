// Copyright 2017 The dad-go Authors
// This file is part of the dad-go library.
//
// The dad-go library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The dad-go library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the dad-go library. If not, see <http://www.gnu.org/licenses/>.

package smartcontract

import (
	"bytes"

	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/core/store"
	scommon "github.com/ontio/dad-go/core/store/common"
	ctypes "github.com/ontio/dad-go/core/types"
	"github.com/ontio/dad-go/errors"
	"github.com/ontio/dad-go/smartcontract/context"
	"github.com/ontio/dad-go/smartcontract/event"
	"github.com/ontio/dad-go/smartcontract/service/native"
	stypes "github.com/ontio/dad-go/smartcontract/types"
	"github.com/ontio/dad-go/smartcontract/service/neovm"
	"github.com/ontio/dad-go/core/payload"
	"github.com/ontio/dad-go/smartcontract/states"
	vm "github.com/ontio/dad-go/vm/neovm"
	"github.com/ontio/dad-go/smartcontract/service/wasmvm"
)

var (
	CONTRACT_NOT_EXIST = errors.NewErr("[AppCall] Get contract context nil")
	DEPLOYCODE_TYPE_ERROR = errors.NewErr("[AppCall] DeployCode type error!")
	INVOKE_CODE_EXIST = errors.NewErr("[AppCall] Invoke codes exist!")
	ENGINE_NOT_SUPPORT = errors.NewErr("[Execute] Engine doesn't support!")
)

// SmartContract describe smart contract execute engine
type SmartContract struct {
	Contexts      []*context.Context       // all execute smart contract context
	Config        *Config
	Engine        Engine
	Notifications []*event.NotifyEventInfo // all execute smart contract event notify info
}

// Config describe smart contract need parameters configuration
type Config struct {
	Time    uint32              // current block timestamp
	Height  uint32              // current block height
	Tx      *ctypes.Transaction // current transaction
	DBCache scommon.StateStore  // db states cache
	Store   store.LedgerStore   // ledger store
}

type Engine interface {
	Invoke() (interface{}, error)
}

// PushContext push current context to smart contract
func (this *SmartContract) PushContext(context *context.Context) {
	this.Contexts = append(this.Contexts, context)
}

// CurrentContext return smart contract current context
func (this *SmartContract) CurrentContext() *context.Context {
	if len(this.Contexts) < 1 {
		return nil
	}
	return this.Contexts[len(this.Contexts) - 1]
}

// CallingContext return smart contract caller context
func (this *SmartContract) CallingContext() *context.Context {
	if len(this.Contexts) < 2 {
		return nil
	}
	return this.Contexts[len(this.Contexts) - 2]
}

// EntryContext return smart contract entry entrance context
func (this *SmartContract) EntryContext() *context.Context {
	if len(this.Contexts) < 1 {
		return nil
	}
	return this.Contexts[0]
}

// PopContext pop smart contract current context
func (this *SmartContract) PopContext() {
	if len(this.Contexts) > 0 {
		this.Contexts = this.Contexts[:len(this.Contexts) - 1]
	}
}

// PushNotifications push smart contract event info
func (this *SmartContract) PushNotifications(notifications []*event.NotifyEventInfo) {
	this.Notifications = append(this.Notifications, notifications...)
}

// Execute is smart contract execute manager
// According different vm type to launch different service
func (this *SmartContract) Execute() (interface{}, error) {
	ctx := this.CurrentContext()
	var engine Engine
	switch ctx.Code.VmType {
	case stypes.Native:
		engine = native.NewNativeService(this.Config.DBCache, this.Config.Height, this.Config.Tx, this)
	case stypes.NEOVM:
		engine = neovm.NewNeoVmService(this.Config.Store, this.Config.DBCache, this.Config.Tx, this.Config.Time, this)
	case stypes.WASMVM:
		engine = wasmvm.NewWasmVmService(this.Config.Store, this.Config.DBCache, this.Config.Tx, this.Config.Time, this)
	default:
		return nil, ENGINE_NOT_SUPPORT
	}
	return engine.Invoke()
}

// AppCall a smart contract, if contract exist on blockchain, you should set the address
// Param address: invoke smart contract on blockchain according contract address
// Param method: invoke smart contract method name
// Param codes: invoke smart contract off blockchain
// Param args: invoke smart contract args
func (this *SmartContract) AppCall(address common.Address, method string, codes, args []byte) (interface{}, error) {
	var code []byte

	vmType := stypes.VmType(address[0])

	switch vmType {
	case stypes.Native:
		bf := new(bytes.Buffer)
		c := states.Contract{
			Address: address,
			Method: method,
			Args: args,
		}
		if err := c.Serialize(bf); err != nil {
			return nil, err
		}
		code = bf.Bytes()
	case stypes.NEOVM:
		c, err := this.loadCode(address, codes)
		if err != nil {
			return nil, err
		}
		var temp []byte
		build := vm.NewParamsBuilder(new(bytes.Buffer))
		if method != "" {
			build.EmitPushByteArray([]byte(method))
		}
		temp = append(args, build.ToArray()...)
		code = append(temp, c...)
	case stypes.WASMVM:
		c, err := this.loadCode(address, codes)
		if err != nil {
			return nil, err
		}
		bf := new(bytes.Buffer)
		contract := states.Contract{
			Version: 1, //fix to > 0
			Address: address,
			Method: method,
			Args: args,
			Code: c,
		}
		if err := contract.Serialize(bf); err != nil {
			return nil, err
		}
		code = bf.Bytes()
	}

	this.PushContext(&context.Context{
		Code: stypes.VmCode{
			Code: code,
			VmType: vmType,
		},
		ContractAddress: address,
	})
	res, err := this.Execute()
	if err != nil {
		return nil, err
	}
	this.PopContext()
	return res, nil
}

// CheckWitness check whether authorization correct
// If address is wallet address, check whether in the signature addressed list
// Else check whether address is calling contract address
// Param address: wallet address or contract address
func (this *SmartContract) CheckWitness(address common.Address) bool {
	if stypes.IsVmCodeAddress(address) {
		if this.CallingContext() != nil && this.CallingContext().ContractAddress == address {
			return true
		}
	} else {
		addresses := this.Config.Tx.GetSignatureAddresses()
		for _, v := range addresses {
			if v == address {
				return true
			}
		}
	}

	return false
}

// loadCode load smart contract execute code
// Param address, invoke on blockchain smart contract address
// Param codes, invoke off blockchain smart contract code
// If you invoke off blockchain smart contract, you can set address is codes address
// But this address doesn't deployed on blockchain
func (this *SmartContract) loadCode(address common.Address, codes []byte) ([]byte, error) {
	isLoad := false
	if len(codes) == 0 {
		isLoad = true
	}
	item, err := this.getContract(address[:]); if err != nil {
		return nil, err
	}
	if isLoad {
		if item == nil {
			return nil, CONTRACT_NOT_EXIST
		}
		contract, ok := item.Value.(*payload.DeployCode); if !ok {
			return nil, DEPLOYCODE_TYPE_ERROR
		}
		return contract.Code.Code, nil
	} else {
		if item != nil {
			return nil, INVOKE_CODE_EXIST
		}
		return codes, nil
	}
}

func (this *SmartContract) getContract(address []byte) (*scommon.StateItem, error) {
	item, err := this.Config.DBCache.TryGet(scommon.ST_CONTRACT, address[:]);
	if err != nil {
		return nil, errors.NewErr("[getContract] Get contract context error!")
	}
	return item, nil
}
