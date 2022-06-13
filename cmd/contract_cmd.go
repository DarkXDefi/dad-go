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

package cmd

import (
	"encoding/json"
	"fmt"
	cmdcom "github.com/ontio/dad-go/cmd/common"
	"github.com/ontio/dad-go/cmd/utils"
	"github.com/ontio/dad-go/common"
	"github.com/ontio/dad-go/common/config"
	"github.com/ontio/dad-go/core/payload"
	httpcom "github.com/ontio/dad-go/http/base/common"
	"github.com/ontio/dad-go/smartcontract/states"
	"github.com/urfave/cli"
	"io/ioutil"
	"strings"
)

var (
	ContractCommand = cli.Command{
		Name:        "contract",
		Action:      cli.ShowSubcommandHelp,
		Usage:       "Deploy or invoke smart contract",
		ArgsUsage:   " ",
		Description: `Smart contract operations support the deployment of NeoVM / WasmVM smart contract, and the pre-execution and execution of NeoVM / WasmVM smart contract.`,
		Subcommands: []cli.Command{
			{
				Action:    deployContract,
				Name:      "deploy",
				Usage:     "Deploy a smart contract to dad-go",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					utils.RPCPortFlag,
					utils.TransactionGasPriceFlag,
					utils.TransactionGasLimitFlag,
					utils.ContractVmTypeFlag,
					utils.ContractCodeFileFlag,
					utils.ContractNameFlag,
					utils.ContractVersionFlag,
					utils.ContractAuthorFlag,
					utils.ContractEmailFlag,
					utils.ContractDescFlag,
					utils.ContractPrepareDeployFlag,
					utils.WalletFileFlag,
					utils.AccountAddressFlag,
				},
			},
			{
				Action: invokeContract,
				Name:   "invoke",
				Usage:  "Invoke smart contract",
				ArgsUsage: `dad-go contract support bytearray(need encode to hex string), string, integer, boolean parameter type.

  Parameter 
     Contract parameters separate with comma ',' to split params. and must add type prefix to params.
     For example:string:foo,int:0,bool:true 
     If parameter is an object array, enclose array with '[]'. 
     For example: string:foo,[int:0,bool:true]

  Note that if string contain some special char like :,[,] and so one, please use '/' char to escape. 
  For example: string:did/:ed1e25c9dccae0c694ee892231407afa20b76008

  Return type
     When invoke contract with --prepare flag, you need specifies return type by --return flag, to decode the return value.
     Return type support bytearray(encoded to hex string), string, integer, boolean. 
     If return type is object array, enclose array with '[]'. 
     For example: [string,int,bool,string]
`,
				Flags: []cli.Flag{
					utils.RPCPortFlag,
					utils.TransactionGasPriceFlag,
					utils.TransactionGasLimitFlag,
					utils.ContractAddrFlag,
					utils.ContractVmTypeFlag,
					utils.ContractParamsFlag,
					utils.ContractVersionFlag,
					utils.ContractPrepareInvokeFlag,
					utils.ContractReturnTypeFlag,
					utils.WalletFileFlag,
					utils.AccountAddressFlag,
				},
			},
			{
				Action:    invokeCodeContract,
				Name:      "invokecode",
				Usage:     "Invoke smart contract by code",
				ArgsUsage: " ",
				Flags: []cli.Flag{
					utils.RPCPortFlag,
					utils.ContractCodeFileFlag,
					utils.TransactionGasPriceFlag,
					utils.TransactionGasLimitFlag,
					utils.WalletFileFlag,
					utils.ContractPrepareInvokeFlag,
					utils.AccountAddressFlag,
				},
			},
		},
	}
)

func deployContract(ctx *cli.Context) error {
	SetRpcPort(ctx)
	if !ctx.IsSet(utils.GetFlagName(utils.ContractCodeFileFlag)) ||
		!ctx.IsSet(utils.GetFlagName(utils.ContractNameFlag)) {
		PrintErrorMsg("Missing %s or %s argument.", utils.ContractCodeFileFlag.Name, utils.ContractNameFlag.Name)
		cli.ShowSubcommandHelp(ctx)
		return nil
	}

	vmtype := ctx.Uint(utils.GetFlagName(utils.ContractVmTypeFlag))
	codeFile := ctx.String(utils.GetFlagName(utils.ContractCodeFileFlag))
	if "" == codeFile {
		return fmt.Errorf("please specific code file")
	}
	codeStr, err := ioutil.ReadFile(codeFile)
	if err != nil {
		return fmt.Errorf("read code:%s error:%s", codeFile, err)
	}

	name := ctx.String(utils.GetFlagName(utils.ContractNameFlag))
	version := ctx.String(utils.GetFlagName(utils.ContractVersionFlag))
	author := ctx.String(utils.GetFlagName(utils.ContractAuthorFlag))
	email := ctx.String(utils.GetFlagName(utils.ContractEmailFlag))
	desc := ctx.String(utils.GetFlagName(utils.ContractDescFlag))
	code := strings.TrimSpace(string(codeStr))
	gasPrice := ctx.Uint64(utils.GetFlagName(utils.TransactionGasPriceFlag))
	gasLimit := ctx.Uint64(utils.GetFlagName(utils.TransactionGasLimitFlag))
	networkId, err := utils.GetNetworkId()
	if err != nil {
		return err
	}
	if networkId == config.NETWORK_ID_SOLO_NET {
		gasPrice = 0
	}

	cversion := fmt.Sprintf("%s", version)

	if ctx.IsSet(utils.GetFlagName(utils.ContractPrepareDeployFlag)) {
		preResult, err := utils.PrepareDeployContract(byte(vmtype), code, name, cversion, author, email, desc)
		if err != nil {
			return fmt.Errorf("PrepareDeployContract error:%s", err)
		}
		if preResult.State == 0 {
			return fmt.Errorf("contract pre-deploy failed")
		}
		PrintInfoMsg("Contract pre-deploy successfully.")
		PrintInfoMsg("Gas consumed:%d.", preResult.Gas)
		return nil
	}

	signer, err := cmdcom.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("get signer account error:%s", err)
	}

	txHash, err := utils.DeployContract(gasPrice, gasLimit, signer, byte(vmtype), code, name, cversion, author, email, desc)
	if err != nil {
		return fmt.Errorf("DeployContract error:%s", err)
	}
	c, _ := common.HexToBytes(code)
	address := common.AddressFromVmCode(c)
	PrintInfoMsg("Deploy contract:")
	PrintInfoMsg("  Contract Address:%s", address.ToHexString())
	PrintInfoMsg("  TxHash:%s", txHash)
	PrintInfoMsg("\nTip:")
	PrintInfoMsg("  Using './dad-go info status %s' to query transaction status.", txHash)
	return nil
}

func invokeCodeContract(ctx *cli.Context) error {
	SetRpcPort(ctx)
	if !ctx.IsSet(utils.GetFlagName(utils.ContractCodeFileFlag)) {
		PrintErrorMsg("Missing %s or %s argument.", utils.ContractCodeFileFlag.Name, utils.ContractNameFlag.Name)
		cli.ShowSubcommandHelp(ctx)
		return nil
	}

	codeFile := ctx.String(utils.GetFlagName(utils.ContractCodeFileFlag))
	if "" == codeFile {
		return fmt.Errorf("please specific code file")
	}
	codeStr, err := ioutil.ReadFile(codeFile)
	if err != nil {
		return fmt.Errorf("read code:%s error:%s", codeFile, err)
	}
	code := strings.TrimSpace(string(codeStr))
	c, err := common.HexToBytes(code)
	if err != nil {
		return fmt.Errorf("contrace code convert hex to bytes error:%s", err)
	}

	if ctx.IsSet(utils.GetFlagName(utils.ContractPrepareInvokeFlag)) {
		preResult, err := utils.PrepareInvokeCodeNeoVMContract(c)
		if err != nil {
			return fmt.Errorf("PrepareInvokeCodeNeoVMContract error:%s", err)
		}
		if preResult.State == 0 {
			return fmt.Errorf("contract pre-invoke failed")
		}
		PrintInfoMsg("Contract pre-invoke successfully")
		PrintInfoMsg("  Gas limit:%d", preResult.Gas)

		rawReturnTypes := ctx.String(utils.GetFlagName(utils.ContractReturnTypeFlag))
		if rawReturnTypes == "" {
			PrintInfoMsg("Return:%s (raw value)", preResult.Result)
			return nil
		}
		values, err := utils.ParseReturnValue(preResult.Result, rawReturnTypes, payload.NEOVM_TYPE)
		if err != nil {
			return fmt.Errorf("parseReturnValue values:%+v types:%s error:%s", values, rawReturnTypes, err)
		}
		switch len(values) {
		case 0:
			PrintInfoMsg("Return: nil")
		case 1:
			PrintInfoMsg("Return:%+v", values[0])
		default:
			PrintInfoMsg("Return:%+v", values)
		}
		return nil
	}
	gasPrice := ctx.Uint64(utils.GetFlagName(utils.TransactionGasPriceFlag))
	gasLimit := ctx.Uint64(utils.GetFlagName(utils.TransactionGasLimitFlag))
	networkId, err := utils.GetNetworkId()
	if err != nil {
		return err
	}
	if networkId == config.NETWORK_ID_SOLO_NET {
		gasPrice = 0
	}

	invokeTx, err := httpcom.NewSmartContractTransaction(gasPrice, gasLimit, c)
	if err != nil {
		return err
	}

	signer, err := cmdcom.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("get signer account error:%s", err)
	}

	err = utils.SignTransaction(signer, invokeTx)
	if err != nil {
		return fmt.Errorf("SignTransaction error:%s", err)
	}
	tx, err := invokeTx.IntoImmutable()
	if err != nil {
		return err
	}

	txHash, err := utils.SendRawTransaction(tx)
	if err != nil {
		return fmt.Errorf("SendTransaction error:%s", err)
	}

	PrintInfoMsg("TxHash:%s", txHash)
	PrintInfoMsg("\nTip:")
	PrintInfoMsg("  Using './dad-go info status %s' to query transaction status.", txHash)
	return nil
}

func invokeContract(ctx *cli.Context) error {
	SetRpcPort(ctx)
	if !ctx.IsSet(utils.GetFlagName(utils.ContractAddrFlag)) {
		PrintErrorMsg("Missing %s argument.", utils.ContractAddrFlag.Name)
		cli.ShowSubcommandHelp(ctx)
		return nil
	}
	contractAddrStr := ctx.String(utils.GetFlagName(utils.ContractAddrFlag))
	contractAddr, err := common.AddressFromHexString(contractAddrStr)
	if err != nil {
		return fmt.Errorf("invalid contract address error:%s", err)
	}
	vmtype := ctx.Uint(utils.GetFlagName(utils.ContractVmTypeFlag))
	if byte(vmtype) != payload.NEOVM_TYPE && byte(vmtype) != payload.WASMVM_TYPE {
		return fmt.Errorf("invalid vmtype")
	}
	paramsStr := ctx.String(utils.GetFlagName(utils.ContractParamsFlag))
	params, err := utils.ParseParams(paramsStr)
	if err != nil {
		return fmt.Errorf("parseParams error:%s", err)
	}

	paramData, _ := json.Marshal(params)
	PrintInfoMsg("Invoke:%x Params:%s", contractAddr[:], paramData)
	if ctx.IsSet(utils.GetFlagName(utils.ContractPrepareInvokeFlag)) {

		var preResult *states.PreExecResult
		if byte(vmtype) == payload.NEOVM_TYPE {
			preResult, err = utils.PrepareInvokeNeoVMContract(contractAddr, params)

		}
		if byte(vmtype) == payload.WASMVM_TYPE {
			preResult, err = utils.PrepareInvokeWasmVMContract(contractAddr, params)
		}

		if err != nil {
			return fmt.Errorf("PrepareInvokeNeoVMSmartContact error:%s", err)
		}
		if preResult.State == 0 {
			return fmt.Errorf("contract invoke failed")
		}

		PrintInfoMsg("Contract invoke successfully")
		PrintInfoMsg("  Gas limit:%d", preResult.Gas)

		rawReturnTypes := ctx.String(utils.GetFlagName(utils.ContractReturnTypeFlag))
		if rawReturnTypes == "" {
			PrintInfoMsg("  Return:%s (raw value)", preResult.Result)
			return nil
		}
		values, err := utils.ParseReturnValue(preResult.Result, rawReturnTypes, byte(vmtype))
		if err != nil {
			return fmt.Errorf("parseReturnValue values:%+v types:%s error:%s", values, rawReturnTypes, err)
		}
		switch len(values) {
		case 0:
			PrintInfoMsg("  Return: nil")
		case 1:
			PrintInfoMsg("  Return:%+v", values[0])
		default:
			PrintInfoMsg("  Return:%+v", values)
		}
		return nil
	}
	signer, err := cmdcom.GetAccount(ctx)
	if err != nil {
		return fmt.Errorf("get signer account error:%s", err)
	}
	gasPrice := ctx.Uint64(utils.GetFlagName(utils.TransactionGasPriceFlag))
	gasLimit := ctx.Uint64(utils.GetFlagName(utils.TransactionGasLimitFlag))
	networkId, err := utils.GetNetworkId()
	if err != nil {
		return err
	}
	if networkId == config.NETWORK_ID_SOLO_NET {
		gasPrice = 0
	}

	var txHash string
	if byte(vmtype) == payload.NEOVM_TYPE {
		txHash, err = utils.InvokeNeoVMContract(gasPrice, gasLimit, signer, contractAddr, params)
		if err != nil {
			return fmt.Errorf("invoke NeoVM contract error:%s", err)
		}
	}
	if byte(vmtype) == payload.WASMVM_TYPE {
		txHash, err = utils.InvokeWasmVMContract(gasPrice, gasLimit, signer, contractAddr, params)
		if err != nil {
			return fmt.Errorf("invoke NeoVM contract error:%s", err)
		}
	}

	PrintInfoMsg("  TxHash:%s", txHash)
	PrintInfoMsg("\nTips:")
	PrintInfoMsg("  Using './dad-go info status %s' to query transaction status.", txHash)
	return nil
}
