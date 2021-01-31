/*
* Copyright 2020, Offchain Labs, Inc.
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You may obtain a copy of the License at
*
*    http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
 */

package arbostest

import (
	"github.com/offchainlabs/arbitrum/packages/arb-util/arbos"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/offchainlabs/arbitrum/packages/arb-evm/evm"
	"github.com/offchainlabs/arbitrum/packages/arb-evm/message"
	"github.com/offchainlabs/arbitrum/packages/arb-rpc-node/arbostestcontracts"
	"github.com/offchainlabs/arbitrum/packages/arb-rpc-node/snapshot"
	"github.com/offchainlabs/arbitrum/packages/arb-util/common"
	"github.com/offchainlabs/arbitrum/packages/arb-util/inbox"
)

func TestFib(t *testing.T) {
	chainTime := inbox.ChainTime{
		BlockNum:  common.NewTimeBlocksInt(0),
		Timestamp: big.NewInt(0),
	}

	fib, err := abi.JSON(strings.NewReader(arbostestcontracts.FibonacciABI))
	failIfError(t, err)

	constructorData, err := hexutil.Decode(arbostestcontracts.FibonacciBin)
	failIfError(t, err)

	constructTx := message.Transaction{
		MaxGas:      big.NewInt(1000000000),
		GasPriceBid: big.NewInt(0),
		SequenceNum: big.NewInt(0),
		DestAddress: common.Address{},
		Payment:     big.NewInt(0),
		Data:        constructorData,
	}

	generateTx := message.Transaction{
		MaxGas:      big.NewInt(1000000000),
		GasPriceBid: big.NewInt(0),
		SequenceNum: big.NewInt(1),
		DestAddress: connAddress1,
		Payment:     big.NewInt(300),
		Data:        generateFib(t, big.NewInt(20)),
	}

	getFibTx := message.Call{
		BasicTx: message.BasicTx{
			MaxGas:      big.NewInt(1000000000),
			GasPriceBid: big.NewInt(0),
			DestAddress: connAddress1,
			Payment:     big.NewInt(0),
			Data:        makeFuncData(t, fib.Methods["getFib"], big.NewInt(5)),
		},
	}

	inboxMessages := makeSimpleInbox([]message.Message{
		message.NewSafeL2Message(constructTx),
		message.Eth{
			Dest:  sender,
			Value: big.NewInt(1000),
		},
		message.NewSafeL2Message(generateTx),
		message.NewSafeL2Message(getFibTx),
	})

	logs, _, mach, _ := runAssertion(t, inboxMessages, 3, 0)
	results := processTxResults(t, logs)
	allResultsSucceeded(t, results)
	checkConstructorResult(t, results[0], connAddress1)

	generateResult := results[1]
	if len(generateResult.EVMLogs) != 1 {
		t.Fatal("incorrect log count")
	}
	evmLog := generateResult.EVMLogs[0]
	if evmLog.Address != connAddress1 {
		t.Fatal("log came from incorrect address")
	}
	if evmLog.Topics[0].ToEthHash() != fib.Events["TestEvent"].ID {
		t.Fatal("incorrect log topic")
	}
	if hexutil.Encode(evmLog.Data) != "0x0000000000000000000000000000000000000000000000000000000000000014" {
		t.Fatal("incorrect log data")
	}

	if hexutil.Encode(results[2].ReturnData) != "0x0000000000000000000000000000000000000000000000000000000000000008" {
		t.Fatal("getFib had incorrect result")
	}

	snap := snapshot.NewSnapshot(mach.Clone(), chainTime, message.ChainAddressToID(chain), big.NewInt(1))
	code, err := snap.GetCode(connAddress1)
	failIfError(t, err)
	t.Log("code", len(code))

}

func TestDeposit(t *testing.T) {
	chainTime := inbox.ChainTime{
		BlockNum:  common.NewTimeBlocksInt(0),
		Timestamp: big.NewInt(0),
	}

	amount := big.NewInt(1000)
	messages := []message.Message{
		message.Eth{
			Dest:  sender,
			Value: amount,
		},
	}

	_, _, mach, _ := runAssertion(t, makeSimpleInbox(messages), 0, 0)

	snap := snapshot.NewSnapshot(mach.Clone(), chainTime, message.ChainAddressToID(chain), big.NewInt(1))
	checkBalance(t, snap, sender, amount)
}

func TestBlocks(t *testing.T) {
	messages := make([]inbox.InboxMessage, 0)
	startTime := inbox.ChainTime{
		BlockNum:  common.NewTimeBlocksInt(0),
		Timestamp: big.NewInt(0),
	}

	messages = append(
		messages,
		message.NewInboxMessage(initMsg(), chain, big.NewInt(0), startTime),
	)

	messages = append(
		messages,
		message.NewInboxMessage(message.Eth{Value: big.NewInt(100), Dest: sender}, chain, big.NewInt(0), startTime),
	)

	blockTimes := make([]inbox.ChainTime, 0)
	for i := int64(0); i < 5; i++ {
		time := inbox.ChainTime{
			BlockNum:  common.NewTimeBlocksInt(1 + i),
			Timestamp: big.NewInt(10 + i),
		}
		blockTimes = append(blockTimes, time)
	}

	for i := int64(0); i < 5; i++ {
		tx := message.Transaction{
			MaxGas:      big.NewInt(100000000000),
			GasPriceBid: big.NewInt(0),
			SequenceNum: big.NewInt(i * 2),
			DestAddress: common.NewAddressFromEth(arbos.ARB_SYS_ADDRESS),
			Payment:     big.NewInt(i * 2),
			Data:        snapshot.WithdrawEthData(common.Address{}),
		}
		tx2 := message.Transaction{
			MaxGas:      big.NewInt(100000000000),
			GasPriceBid: big.NewInt(0),
			SequenceNum: big.NewInt(i*2 + 1),
			DestAddress: common.NewAddressFromEth(arbos.ARB_SYS_ADDRESS),
			Payment:     big.NewInt(i*2 + 1),
			Data:        snapshot.WithdrawEthData(common.Address{}),
		}
		messages = append(
			messages,
			message.NewInboxMessage(
				message.NewSafeL2Message(tx),
				sender,
				big.NewInt(i*2+2),
				blockTimes[i],
			),
		)
		messages = append(
			messages,
			message.NewInboxMessage(
				message.NewSafeL2Message(tx2),
				sender,
				big.NewInt(i*2+2),
				blockTimes[i],
			),
		)
	}

	// Last value returned is not an error type
	avmLogs, sends, _, _ := runAssertion(t, messages, 14, 10)
	results := make([]evm.Result, 0)
	for _, avmLog := range avmLogs {
		res, err := evm.NewResultFromValue(avmLog)
		failIfError(t, err)
		results = append(results, res)
	}

	blockGasUsed := big.NewInt(0)
	blockAVMLogCount := big.NewInt(0)
	blockEVMLogCount := big.NewInt(0)
	blockTxCount := big.NewInt(0)

	totalGasUsed := big.NewInt(0)
	totalAVMLogCount := big.NewInt(0)
	totalEVMLogCount := big.NewInt(0)
	totalTxCount := big.NewInt(0)
	blockCount := 0
	prevBlockNum := abi.MaxUint256

	blocks := make([]*evm.BlockInfo, 0)

	for i, res := range results {
		totalAVMLogCount = totalAVMLogCount.Add(totalAVMLogCount, big.NewInt(1))

		if i%3 == 0 || i%3 == 1 {
			res, ok := res.(*evm.TxResult)
			if !ok {
				t.Error("incorrect result type")
			}
			succeededTxCheck(t, res)
			blockGasUsed = blockGasUsed.Add(blockGasUsed, res.GasUsed)
			blockEVMLogCount = blockEVMLogCount.Add(blockEVMLogCount, big.NewInt(int64(len(res.EVMLogs))))
			blockTxCount = blockTxCount.Add(blockTxCount, big.NewInt(1))
			blockAVMLogCount = blockAVMLogCount.Add(blockAVMLogCount, big.NewInt(1))

			totalGasUsed = totalGasUsed.Add(totalGasUsed, res.GasUsed)
			totalEVMLogCount = totalEVMLogCount.Add(totalEVMLogCount, big.NewInt(int64(len(res.EVMLogs))))
			totalTxCount = totalTxCount.Add(totalTxCount, big.NewInt(1))
		} else {
			res, ok := res.(*evm.BlockInfo)
			if !ok {
				t.Fatal("incorrect result type")
			}
			blocks = append(blocks, res)

			correctTime := blockTimes[blockCount]
			if res.BlockNum.Cmp(correctTime.BlockNum.AsInt()) != 0 {
				t.Error("unexpected block height", res.BlockNum, i)
			}
			if res.Timestamp.Cmp(correctTime.Timestamp) != 0 {
				t.Error("unexpected timestamp", res.Timestamp, 10+i)
			}

			if res.BlockStats.GasUsed.Cmp(blockGasUsed) != 0 {
				t.Error("unexpected chain gas used")
			}
			if res.BlockStats.AVMLogCount.Cmp(blockAVMLogCount) != 0 {
				t.Error("unexpected block log count", res.BlockStats.AVMLogCount, "instead of", blockAVMLogCount)
			}
			if res.BlockStats.AVMSendCount.Cmp(big.NewInt(2)) != 0 {
				t.Error("unexpected block send count")
			}
			if res.BlockStats.EVMLogCount.Cmp(blockEVMLogCount) != 0 {
				t.Error("unexpected block evm log count")
			}
			if res.BlockStats.TxCount.Cmp(blockTxCount) != 0 {
				t.Error("unexpected block tx count", res.BlockStats.TxCount)
			}

			if res.ChainStats.GasUsed.Cmp(totalGasUsed) != 0 {
				t.Error("unexpected chain gas used")
			}
			if res.ChainStats.AVMLogCount.Cmp(totalAVMLogCount) != 0 {
				t.Error("unexpected chain log count", res.ChainStats.AVMLogCount, "instead of", totalAVMLogCount)
			}
			if res.ChainStats.AVMSendCount.Cmp(big.NewInt(int64(blockCount*2)+2)) != 0 {
				t.Error("unexpected chain send count")
			}
			if res.ChainStats.EVMLogCount.Cmp(totalEVMLogCount) != 0 {
				t.Error("unexpected chain evm log count")
			}
			if res.ChainStats.TxCount.Cmp(totalTxCount) != 0 {
				t.Error("unexpected chain tx count", res.ChainStats.TxCount, "instead of", totalTxCount)
			}

			if res.LastAVMLog().Uint64() != uint64(i) {
				t.Error("incorrect last log")
			}

			if res.PreviousHeight.Cmp(prevBlockNum) != 0 {
				t.Error("incorrect prev block num")
			}

			blockGasUsed = big.NewInt(0)
			blockAVMLogCount = big.NewInt(0)
			blockEVMLogCount = big.NewInt(0)
			blockTxCount = big.NewInt(0)
			prevBlockNum = res.BlockNum
			blockCount++
		}
	}

	parsedSends := make([]message.Eth, 0)
	for _, send := range sends {
		outMsg, err := message.NewOutMessageFromValue(send)
		failIfError(t, err)

		if outMsg.Kind != message.EthType {
			t.Fatal("outgoing message had wrong type", outMsg.Kind)
		}

		if outMsg.Sender != sender {
			t.Fatal("wrong withdraw sender")
		}
		parsedSends = append(parsedSends, message.NewEthFromData(outMsg.Data))
	}

	for blockIndex, block := range blocks {
		txCount := block.BlockStats.TxCount.Uint64()
		startLog := block.FirstAVMLog().Uint64()
		for i := uint64(0); i < txCount; i++ {
			txRes, ok := results[startLog+i].(*evm.TxResult)
			if !ok {
				t.Fatal("block results must be tx results")
			}
			if txRes.IncomingRequest.ChainTime.BlockNum.AsInt().Cmp(block.BlockNum) != 0 {
				t.Error("tx in block had wrong block num")
			}
			if txRes.IncomingRequest.ChainTime.Timestamp.Cmp(block.Timestamp) != 0 {
				t.Error("tx in block had wrong timestamp")
			}
		}

		sendCount := block.BlockStats.AVMSendCount.Uint64()
		startSend := block.FirstAVMSend().Uint64()
		if sendCount != 2 {
			t.Fatal("wrong send count")
		}

		for i := uint64(0); i < sendCount; i++ {
			send := parsedSends[startSend+i]
			correctVal := big.NewInt(int64(blockIndex*2 + int(i)))
			if send.Value.Cmp(correctVal) != 0 {
				t.Log("block", blockIndex)
				t.Log("index in block", i)
				t.Log("log index", startSend+i)
				t.Log("send value", send.Value)
				t.Log("correct", correctVal)
				t.Fatal("wrong send value")
			}
		}
	}
}
