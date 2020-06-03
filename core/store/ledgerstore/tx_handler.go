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

package ledgerstore

import (
	"bytes"
	"fmt"
	"github.com/dad-go/core/payload"
	"github.com/dad-go/core/states"
	scommon "github.com/dad-go/core/store/common"
	"github.com/dad-go/common"
	"github.com/dad-go/core/store/statestore"
	"github.com/dad-go/core/types"
	vmtypes "github.com/dad-go/vm/types"
	"github.com/dad-go/smartcontract"
	"github.com/dad-go/core/store"
	"github.com/dad-go/smartcontract/context"
	"github.com/dad-go/smartcontract/event"
)

const (
	INVOKE_TRANSACTION = "InvokeTransaction"
)

func (this *StateStore) HandleDeployTransaction(stateBatch *statestore.StateBatch, tx *types.Transaction) error {
	deploy := tx.Payload.(*payload.DeployCode)

	originAddress := deploy.Code.AddressFromVmCode()

	// mapping native contract origin address to target address
	if deploy.Code.VmType == vmtypes.Native {
		targetAddress, err := common.AddressParseFromBytes(deploy.Code.Code)
		if err != nil {
			return fmt.Errorf("Invalid native contract address:%v", err)
		}
		if err := stateBatch.TryGetOrAdd(
			scommon.ST_CONTRACT,
			targetAddress[:],
			&states.ContractMapping{
				OriginAddress: originAddress,
				TargetAddress: targetAddress,
			},
			false); err != nil {
			return fmt.Errorf("TryGetOrAdd contract error %s", err)
		}
	}

	// store contract message
	if err := stateBatch.TryGetOrAdd(
		scommon.ST_CONTRACT,
		originAddress[:],
		deploy,
		false); err != nil {
		return fmt.Errorf("TryGetOrAdd contract error %s", err)
	}
	return nil
}

func (this *StateStore) HandleInvokeTransaction(store store.LedgerStore, stateBatch *statestore.StateBatch, tx *types.Transaction, block *types.Block, eventStore scommon.EventStore) error {
	invoke := tx.Payload.(*payload.InvokeCode)
	txHash := tx.Hash()

	// init smart contract configuration info
	config := &smartcontract.Config{
		Time: block.Header.Timestamp,
		Height: block.Header.Height,
		Tx: tx,
		Table: &CacheCodeTable{stateBatch},
		DBCache: stateBatch,
		Store: store,
	}

	//init smart contract context info
	ctx := &context.Context{
		Code: invoke.Code,
		ContractAddress: invoke.Code.AddressFromVmCode(),
	}

	//init smart contract info
	sc := smartcontract.SmartContract{
		Config: config,
	}

	//load current context to smart contract
	sc.PushContext(ctx)

	//start the smart contract executive function
	if err := sc.Execute(); err != nil {
		return err
	}

	if len(sc.Notifications) > 0 {
		if err := eventStore.SaveEventNotifyByTx(txHash, sc.Notifications); err != nil {
			return fmt.Errorf("SaveEventNotifyByTx error %s", err)
		}
		event.PushSmartCodeEvent(txHash, 0, INVOKE_TRANSACTION, sc.Notifications)
	}
	return nil
}

func (this *StateStore) HandleClaimTransaction(stateBatch *statestore.StateBatch, tx *types.Transaction) error {
	//TODO
	return nil
}

func (this *StateStore) HandleVoteTransaction(stateBatch *statestore.StateBatch, tx *types.Transaction) error {
	vote := tx.Payload.(*payload.Vote)
	buf := new(bytes.Buffer)
	vote.Account.Serialize(buf)
	stateBatch.TryAdd(scommon.ST_VOTE, buf.Bytes(), &states.VoteState{PublicKeys: vote.PubKeys}, false)
	return nil
}




