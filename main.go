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

package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/ontio/dad-go-crypto/keypair"
	"github.com/ontio/dad-go/account"
	"github.com/ontio/dad-go/cmd"
	"github.com/ontio/dad-go/cmd/utils"
	"github.com/ontio/dad-go/common/config"
	"github.com/ontio/dad-go/common/log"
	"github.com/ontio/dad-go/common/password"
	"github.com/ontio/dad-go/consensus"
	"github.com/ontio/dad-go/core/ledger"
	ldgactor "github.com/ontio/dad-go/core/ledger/actor"
	"github.com/ontio/dad-go/events"
	hserver "github.com/ontio/dad-go/http/base/actor"
	"github.com/ontio/dad-go/http/jsonrpc"
	"github.com/ontio/dad-go/http/localrpc"
	"github.com/ontio/dad-go/http/nodeinfo"
	"github.com/ontio/dad-go/http/restful"
	"github.com/ontio/dad-go/http/websocket"
	"github.com/ontio/dad-go/net"
	"github.com/ontio/dad-go/net/protocol"
	"github.com/ontio/dad-go/txnpool"
	tc "github.com/ontio/dad-go/txnpool/common"
	"github.com/ontio/dad-go/validator/statefull"
	"github.com/ontio/dad-go/validator/stateless"
	"github.com/urfave/cli"
)

const (
	DefaultMultiCoreNum = 4
)

func init() {
	log.Init(log.PATH, log.Stdout)
	//cmd.HelpUsage()
	// Todo: If the actor bus uses a different log lib, remove it

	var coreNum int
	if config.Parameters.MultiCoreNum > DefaultMultiCoreNum {
		coreNum = int(config.Parameters.MultiCoreNum)
	} else {
		coreNum = DefaultMultiCoreNum
	}
	log.Debug("The Core number is ", coreNum)
	runtime.GOMAXPROCS(coreNum)
}

func setupAPP() *cli.App {
	app := cli.NewApp()
	app.Usage = "dad-go CLI"
	app.Action = ontMain
	app.Version = "0.7.0"
	app.Copyright = "Copyright in 2018 The dad-go Authors"
	app.Commands = []cli.Command{
		cmd.AccountCommand,
		cmd.InfoCommand,
		cmd.AssetCommand,
		cmd.SettingCommand,
		cmd.ContractCommand,
	}
	app.Flags = []cli.Flag{
		utils.AccountFileFlag,
		utils.AccountPassFlag,
		utils.ConfigUsedFlag,
	}

	return app
}

func main() {
	defer func() {
		if p := recover(); p != nil {
			if str, ok := p.(string); ok {
				log.Warn("Leave gracefully. ", errors.New(str))
			}
		}
	}()

	if err := setupAPP().Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func ontMain(ctx *cli.Context) {
	var acct *account.Account
	var err error
	var noder protocol.Noder

	log.Info("Node version: ", config.Version)
	consensusType := strings.ToLower(config.Parameters.ConsensusType)
	if consensusType == "dbft" && len(config.Parameters.Bookkeepers) < account.DEFAULT_BOOKKEEPER_COUNT {
		log.Fatal("With dbft consensus type, at least ", account.DEFAULT_BOOKKEEPER_COUNT, " Bookkeepers should be set in config.json")
		os.Exit(1)
	}

	log.Info("0. Open the account")
	var pwd []byte = nil
	if ctx.IsSet("password") {
		pwd = []byte(ctx.String("password"))
	} else {
		pwd, err = password.GetAccountPassword()
		if err != nil {
			log.Fatal("Password error")
			os.Exit(1)
		}
	}

	wallet := ctx.GlobalString("file")
	client := account.Open(wallet, pwd)
	if client == nil {
		log.Fatal("Can't get local account.")
		os.Exit(1)
	}
	acct = client.GetDefaultAccount()
	if acct == nil {
		log.Fatal("can not get default account")
		os.Exit(1)
	}
	log.Debug("The Node's PublicKey ", acct.PublicKey)
	defBookkeepers, err := client.GetBookkeepers()
	sort.Sort(keypair.NewPublicList(defBookkeepers))
	if err != nil {
		log.Fatalf("GetBookkeepers error:%s", err)
		os.Exit(1)
	}
	//Init event hub
	events.Init()

	log.Info("1. Loading the Ledger")
	ledger.DefLedger, err = ledger.NewLedger()
	if err != nil {
		log.Fatalf("NewLedger error %s", err)
		os.Exit(1)
	}
	err = ledger.DefLedger.Init(defBookkeepers)
	if err != nil {
		log.Fatalf("DefLedger.Init error %s", err)
		os.Exit(1)
	}
	ldgerActor := ldgactor.NewLedgerActor()
	ledgerPID := ldgerActor.Start()
	log.Info("3. Start the transaction pool server")
	// Start the transaction pool server
	txPoolServer := txnpool.StartTxnPoolServer()
	if txPoolServer == nil {
		log.Fatalf("failed to start txn pool server")
		os.Exit(1)
	}

	stlValidator, _ := stateless.NewValidator("stateless_validator")
	stlValidator.Register(txPoolServer.GetPID(tc.VerifyRspActor))

	stfValidator, _ := statefull.NewValidator("statefull_validator")
	stfValidator.Register(txPoolServer.GetPID(tc.VerifyRspActor))

	log.Info("4. Start the P2P networks")

	net.SetLedgerPid(ledgerPID)
	net.SetTxnPoolPid(txPoolServer.GetPID(tc.TxActor))
	noder = net.StartProtocol(acct.PublicKey)
	if err != nil {
		log.Fatalf("Net StartProtocol error %s", err)
		os.Exit(1)
	}
	p2pActor, err := net.InitNetServerActor(noder)
	if err != nil {
		log.Fatalf("Net InitNetServerActor error %s", err)
		os.Exit(1)
	}

	txPoolServer.RegisterActor(tc.NetActor, p2pActor)

	hserver.SetNetServerPid(p2pActor)
	hserver.SetLedgerPid(ledgerPID)
	hserver.SetTxnPoolPid(txPoolServer.GetPID(tc.TxPoolActor))
	hserver.SetTxPid(txPoolServer.GetPID(tc.TxActor))
	go restful.StartServer()

	if consensusType != "vbft" {
		noder.WaitForPeersStart()
		noder.WaitForSyncBlkFinish()
    }
	if protocol.SERVICE_NODE_NAME != config.Parameters.NodeType {
		log.Info("5. Start Consensus Services")
		pool := txPoolServer.GetPID(tc.TxPoolActor)
		consensusService, _ := consensus.NewConsensusService(acct, pool, ledgerPID, p2pActor)
		net.SetConsensusPid(consensusService.GetPID())
		go consensusService.Start()
		time.Sleep(5 * time.Second)
		hserver.SetConsensusPid(consensusService.GetPID())
		go localrpc.StartLocalServer()
	}

	log.Info("--Start the RPC interface")
	go jsonrpc.StartRPCServer()
	go websocket.StartServer()
	if config.Parameters.HttpInfoPort > 0 {
		go nodeinfo.StartServer(noder)
	}

	go logCurrBlockHeight()

	//等待退出信号
	waitToExit()
}

func logCurrBlockHeight() {
	ticker := time.NewTicker(config.DEFAULT_GEN_BLOCK_TIME * time.Second)
	for {
		select {
		case <-ticker.C:
			log.Infof("BlockHeight = %d", ledger.DefLedger.GetCurrentBlockHeight())
			isNeedNewFile := log.CheckIfNeedNewFile()
			if isNeedNewFile {
				log.ClosePrintLog()
				log.Init(log.PATH, os.Stdout)
			}
		}
	}
}

func waitToExit() {
	exit := make(chan bool, 0)
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	go func() {
		for sig := range sc {
			log.Infof("dad-go received exit signal:%v.", sig.String())
			close(exit)
			break
		}
	}()
	<-exit
}
