/*
Copyright (C) THL A29 Limited, a Tencent company. All rights reserved.

SPDX-License-Identifier: Apache-2.0
*/

package util

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"
	"testing"

	ethabi "chainmaker.org/chainmaker/common/v2/evmutils/abi"
	"github.com/stretchr/testify/require"
)

var abiJson = `
[
  {
    "constant": true,
    "inputs": [],
    "name": "max_order_len",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "accumulation",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "name",
    "outputs": [
      {
        "name": "",
        "type": "string"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_spender",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "approve",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "totalSupply",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "freeze",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "lock",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "referrer",
    "outputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "decimals",
    "outputs": [
      {
        "name": "",
        "type": "uint8"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "zct_withdrawal_limit",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "max_order_amount",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "distAccumulation",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "burn",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_from",
        "type": "address"
      },
      {
        "name": "_to",
        "type": "address"
      }
    ],
    "name": "ownerChangeAccount",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_lock",
        "type": "bool"
      }
    ],
    "name": "lockForAll",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "distributed",
    "outputs": [
      {
        "name": "timestamp",
        "type": "uint256"
      },
      {
        "name": "amount",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "historyProfit",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "lockOf",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "capacity",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "balanceOf",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "lastAccumulation",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "lockAll",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "unfreeze",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "name",
        "type": "string"
      },
      {
        "name": "def",
        "type": "address"
      }
    ],
    "name": "mgrAddressOf",
    "outputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "owner",
    "outputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "symbol",
    "outputs": [
      {
        "name": "",
        "type": "string"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_lock",
        "type": "bool"
      }
    ],
    "name": "lockAccount",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      }
    ],
    "name": "changeAccount",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "todayProfit",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "freezeOf",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "lockAccountOf",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "address"
      },
      {
        "name": "",
        "type": "address"
      }
    ],
    "name": "allowance",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "name": "rate",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "newOwner",
        "type": "address"
      }
    ],
    "name": "transferOwnership",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [],
    "name": "test",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "pure",
    "type": "function"
  },
  {
    "inputs": [
      {
        "name": "_mgr",
        "type": "address"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "constructor"
  },
  {
    "payable": true,
    "stateMutability": "payable",
    "type": "fallback"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "orderId",
        "type": "uint256"
      },
      {
        "indexed": false,
        "name": "len",
        "type": "uint256"
      }
    ],
    "name": "PrepareMint",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": true,
        "name": "to",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "value",
        "type": "uint256"
      }
    ],
    "name": "CommitMint",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "value",
        "type": "uint256"
      }
    ],
    "name": "MintZCT",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": true,
        "name": "to",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "value",
        "type": "uint256"
      }
    ],
    "name": "Transfer",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "_owner",
        "type": "address"
      },
      {
        "indexed": true,
        "name": "_spender",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "Approval",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "value",
        "type": "uint256"
      }
    ],
    "name": "Freeze",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": false,
        "name": "value",
        "type": "uint256"
      }
    ],
    "name": "Unfreeze",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "from",
        "type": "address"
      },
      {
        "indexed": true,
        "name": "to",
        "type": "address"
      }
    ],
    "name": "ChangeAccount",
    "type": "event"
  },
  {
    "anonymous": false,
    "inputs": [
      {
        "indexed": true,
        "name": "previousOwner",
        "type": "address"
      },
      {
        "indexed": true,
        "name": "newOwner",
        "type": "address"
      }
    ],
    "name": "OwnershipTransferred",
    "type": "event"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_limit",
        "type": "uint256"
      }
    ],
    "name": "setWithdrawLimit",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_len",
        "type": "uint256"
      }
    ],
    "name": "setMaxOrderLen",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_amount",
        "type": "uint256"
      }
    ],
    "name": "setMaxOrderAmount",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "receiver",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "mint",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "setCapacity",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_referral",
        "type": "address"
      },
      {
        "name": "_referrer",
        "type": "address"
      }
    ],
    "name": "setReferrer",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_id",
        "type": "uint256"
      },
      {
        "name": "_rate",
        "type": "uint256"
      }
    ],
    "name": "setRate",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_orderId",
        "type": "uint256"
      },
      {
        "name": "_ids",
        "type": "uint256[]"
      },
      {
        "name": "_amount",
        "type": "uint256[]"
      },
      {
        "name": "_customer",
        "type": "address"
      },
      {
        "name": "_merchant",
        "type": "address[]"
      },
      {
        "name": "_len",
        "type": "uint256"
      }
    ],
    "name": "prepareMint",
    "outputs": [
      {
        "name": "",
        "type": "address[]"
      },
      {
        "name": "",
        "type": "uint256[]"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_orderId",
        "type": "uint256"
      }
    ],
    "name": "commitMint",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_orderId",
        "type": "uint256"
      },
      {
        "name": "id",
        "type": "uint256"
      }
    ],
    "name": "commitMintById",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_orderId",
        "type": "uint256[]"
      }
    ],
    "name": "batchCommitMint",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [],
    "name": "dawnTask",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "_address",
        "type": "address"
      }
    ],
    "name": "checkDistribute",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_address",
        "type": "address"
      }
    ],
    "name": "doDistribute",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": true,
    "inputs": [
      {
        "name": "_address",
        "type": "address"
      }
    ],
    "name": "calcDistribution",
    "outputs": [
      {
        "name": "",
        "type": "uint256"
      }
    ],
    "payable": false,
    "stateMutability": "view",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "transfer",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },
  {
    "constant": false,
    "inputs": [
      {
        "name": "_from",
        "type": "address"
      },
      {
        "name": "_to",
        "type": "address"
      },
      {
        "name": "_value",
        "type": "uint256"
      }
    ],
    "name": "transferFrom",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  },{
	"inputs": [{
		"internalType": "address",
		"name": "accountAddress",
		"type": "address"
	}, {
		"internalType": "uint256",
		"name": "id",
		"type": "uint256"
	}, {
		"internalType": "uint256",
		"name": "amount",
		"type": "uint256"
	}, {
		"internalType": "bytes",
		"name": "data",
		"type": "bytes"
	}],
	"name": "mint",
	"outputs": [],
	"stateMutability": "nonpayable",
	"type": "function"
},
  {
    "constant": false,
    "inputs": [
      {
        "name": "_receivers",
        "type": "address[]"
      },
      {
        "name": "_values",
        "type": "uint256[]"
      },
      {
        "name": "_len",
        "type": "uint256"
      }
    ],
    "name": "batchTransfer",
    "outputs": [
      {
        "name": "",
        "type": "bool"
      }
    ],
    "payable": false,
    "stateMutability": "nonpayable",
    "type": "function"
  }
]`

func TestPack(t *testing.T) {
	param := "[{\"uint256\":\"1212\"},{\"uint256[4]\":[\"2\",\"3\",\"4\",\"5\"]},{\"uint256[2]\":[\"39\",\"10\"]},{\"address\":\"0x00192Fb10dF37c9FB26829eb2CC623cd1BF599E8\"},{\"address[4]\":[\"0x00192Fb10dF37c9FB26829eb2CC623cd1BF599E8\",\"0x00192Fb10dF37c9FB26829eb2CC623cd1BF599E8\",\"0x00192Fb10dF37c9FB26829eb2CC623cd1BF599E8\",\"0x00192Fb10dF37c9FB26829eb2CC623cd1BF599E8\"]},{\"uint256\":\"4\"}]"

	abi, err := ethabi.JSON(strings.NewReader(abiJson))
	if err != nil {
		t.Error(err)
	}

	b, err := Pack(abi, "prepareMint", param)
	if err != nil {
		t.Error(err)
	}
	s := hex.EncodeToString(b)
	exp := "75a725cd00000000000000000000000000000000000000000000000000000000000004bc00000000000000000000000000000000000000000000000000000000000000c0000000000000000000000000000000000000000000000000000000000000016000000000000000000000000000192fb10df37c9fb26829eb2cc623cd1bf599e800000000000000000000000000000000000000000000000000000000000001c000000000000000000000000000000000000000000000000000000000000000040000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000500000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000027000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000192fb10df37c9fb26829eb2cc623cd1bf599e800000000000000000000000000192fb10df37c9fb26829eb2cc623cd1bf599e800000000000000000000000000192fb10df37c9fb26829eb2cc623cd1bf599e800000000000000000000000000192fb10df37c9fb26829eb2cc623cd1bf599e8"
	require.Equal(t, exp, s)
}

func TestPack2(t *testing.T) {
	param := "[{\"address\": \"0x08bb3588134dBd89756DBB0B4F46c3A51a9938b9\"},{\"uint256\": \"1\"},{\"uint256\": \"888\"},{\"bytes\": \"0x1234567890\"}]"
	abi, err := ethabi.JSON(strings.NewReader(abiJson))
	if err != nil {
		t.Error(err)
	}

	b, err := Pack(abi, "mint", param)
	if err != nil {
		t.Error(err)
	}
	s := hex.EncodeToString(b)
	exp := "731133e900000000000000000000000008bb3588134dbd89756dbb0b4f46c3a51a9938b900000000000000000000000000000000000000000000000000000000000000010000000000000000000000000000000000000000000000000000000000000378000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000051234567890000000000000000000000000000000000000000000000000000000"
	require.Equal(t, exp, s)
}

// TestParse 类型转换用例测试
func TestParse(t *testing.T) {
	TestDemo("mint", `[{"string[]":["0x5B38Da6a701c568545dCfcB03FcB875f56beddC4", "0x5B38Da6a701c568545dCfcB03FcB875f56beddC4"]}]`)
	TestDemo("mint", `[{"string[2]":["0x5B38Da6a701c568545dCfcB03FcB875f56beddC4", "0x5B38Da6a701c568545dCfcB03FcB875f56beddC4"]}]`)
	TestDemo("transfer", `[{"int8[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int8[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int16[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int16[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int32[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int32[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int64[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"int64[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint8[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint8[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint16[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint16[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint32[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint32[3]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint64[]":[1,2,3]}]`)
	TestDemo("transfer", `[{"uint64[3]":[1,2,3]}]`)
	TestDemo("a", `[{"bytes1[3]":["0x01","0x02","0x03"]}]`)
	TestDemo("a", `[{"bytes2[3]":["0x01","0x02","0x03"]}]`)
	TestDemo("a", `[{"bytes":"C4"}]`)
	TestDemo("a", `[{"address[]":["0x5B38Da6a701c568545dCfcB03FcB875f56beddC4", "0x5B38Da6a701c568545dCfcB03FcB875f56beddC4"]}]`)
	TestDemo("a", `[{"bool[3]":[true,false,true]}]`)
	TestDemo("a", `[{"bool[]":[true,false,true]}]`)
	TestDemo("a", `[{"bool":[true]}]`)
}

const DynamicAbi = `[{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"idList","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"nameList","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"number1List","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"uint256","name":"","type":"uint256"}],"name":"number2List","outputs":[{"internalType":"uint256","name":"","type":"uint256"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"string[]","name":"_idList","type":"string[]"},{"internalType":"string[]","name":"_nameList","type":"string[]"},{"internalType":"uint256[]","name":"_number1List","type":"uint256[]"},{"internalType":"uint256[]","name":"_number2List","type":"uint256[]"}],"name":"setAll","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256[]","name":"_number1List","type":"uint256[]"},{"internalType":"string[]","name":"_idList","type":"string[]"},{"internalType":"uint256[]","name":"_number2List","type":"uint256[]"}],"name":"setNSN","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256[]","name":"_number1List","type":"uint256[]"}],"name":"setOneNumber","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string[]","name":"_idList","type":"string[]"}],"name":"setOneString","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string[]","name":"_idList","type":"string[]"},{"internalType":"uint256[]","name":"_number1List","type":"uint256[]"},{"internalType":"string[]","name":"_nameList","type":"string[]"}],"name":"setSNS","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256[]","name":"_number1List","type":"uint256[]"},{"internalType":"uint256[]","name":"_number2List","type":"uint256[]"}],"name":"setTwoNumber","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string[]","name":"_idList","type":"string[]"},{"internalType":"string[]","name":"_nameList","type":"string[]"}],"name":"setTwoString","outputs":[],"stateMutability":"nonpayable","type":"function"}]`

func TestDynamicStringABI(t *testing.T) {
	parsedAbi0, err := ethabi.JSON(strings.NewReader(`[{"inputs":[],"name":"get","outputs":[{"internalType":"int256[]","name":"","type":"int256[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getAddrs","outputs":[{"internalType":"address[]","name":"","type":"address[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getBools","outputs":[{"internalType":"bool[]","name":"","type":"bool[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getString","outputs":[{"internalType":"string[]","name":"","type":"string[]"}],"stateMutability":"view","type":"function"},{"inputs":[],"name":"getUint","outputs":[{"internalType":"uint256[]","name":"","type":"uint256[]"}],"stateMutability":"view","type":"function"},{"inputs":[{"internalType":"int256[]","name":"dyn","type":"int256[]"}],"name":"set","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"address[]","name":"dyn","type":"address[]"}],"name":"setAddrs","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"bool[]","name":"dyn","type":"bool[]"}],"name":"setBools","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"string[]","name":"dyn","type":"string[]"}],"name":"setStrings","outputs":[],"stateMutability":"nonpayable","type":"function"},{"inputs":[{"internalType":"uint256[]","name":"dyn","type":"uint256[]"}],"name":"setUints","outputs":[],"stateMutability":"nonpayable","type":"function"}]`))
	if err != nil {
		fmt.Println(err)
		return
	}
	//buf := make([]byte, 2)
	//binary.BigEndian.PutUint32(buf, 1)
	data0, err := parsedAbi0.Pack("setAddrs", []string{"0xC7B2776E53caAc66eB0725aF2Dd8B1F54EbFdB94", "0xC7B2776E53caAc66eB0725aF2Dd8B1F54EbFdB94"})
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Printf("%x\n", data0)
	// 解析 ABI
	parsedABI, err := ethabi.JSON(strings.NewReader(DynamicAbi))
	if err != nil {
		fmt.Printf("Failed to parse ABI: %v", err)
		return
	}
	data1, err := parsedABI.Pack("setOneString", []string{"1", "2", "3"})
	if err != nil {
		fmt.Printf("Failed to pack data: %v \n", err)
		return
	}
	expect1 := `25d132a400000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000131000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001320000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013300000000000000000000000000000000000000000000000000000000000000`
	fmt.Println(expect1 == fmt.Sprintf("%x", string(data1)))

	data2, err := parsedABI.Pack("setTwoString", []string{"1", "2", "3"}, []string{"4", "5", "6"})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect2 := `73dfd395000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000001800000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000001310000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000133000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000134000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001350000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013600000000000000000000000000000000000000000000000000000000000000`
	fmt.Println(expect2 == fmt.Sprintf("%x", string(data2)))

	data3, err := parsedABI.Pack("setOneNumber", []uint{1, 2, 3})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect3 := `ff4c2b1100000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003`
	fmt.Println(expect3 == fmt.Sprintf("%x", string(data3)))

	data4, err := parsedABI.Pack("setTwoNumber", []uint{1, 2, 3}, []uint{4, 5, 6})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect4 := `c58be53c000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000c000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006`
	fmt.Println(expect4 == fmt.Sprintf("%x", string(data4)))

	data5, err := parsedABI.Pack("setSNS", []string{"1", "2", "3"}, []uint{4, 5, 6}, []string{"1", "2", "3"})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect5 := `9aa4a42e000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000001a000000000000000000000000000000000000000000000000000000000000002200000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000013100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000132000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001330000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000004000000000000000000000000000000000000000000000000000000000000000500000000000000000000000000000000000000000000000000000000000000060000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000000131000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001320000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013300000000000000000000000000000000000000000000000000000000000000`
	fmt.Println(expect5 == fmt.Sprintf("%x", string(data5)))

	data6, err := parsedABI.Pack("setNSN", []uint{1, 2, 3}, []string{"4", "5", "6"}, []uint{1, 2, 3})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect6 := `2b9c8ef4000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000e0000000000000000000000000000000000000000000000000000000000000022000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000001340000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013500000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000136000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000003`
	fmt.Println(expect6 == fmt.Sprintf("%x", string(data6)))

	data7, err := parsedABI.Pack("setAll", []string{"1", "2", "3"}, []string{"4", "5", "6"}, []uint{1, 2, 3}, []uint{4, 5, 6})
	if err != nil {
		fmt.Printf("Failed to pack data: %v", err)
		return
	}
	expect7 := `6b7036f5000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000001c0000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000003800000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e00000000000000000000000000000000000000000000000000000000000000001310000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000013200000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000133000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000006000000000000000000000000000000000000000000000000000000000000000a000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000013400000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000135000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001360000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000200000000000000000000000000000000000000000000000000000000000000030000000000000000000000000000000000000000000000000000000000000003000000000000000000000000000000000000000000000000000000000000000400000000000000000000000000000000000000000000000000000000000000050000000000000000000000000000000000000000000000000000000000000006`
	fmt.Println(expect7 == fmt.Sprintf("%x", string(data7)))
}

// 长三角json
var cJson = `[
        {
                "inputs": [
                        {
                                "internalType": "string",
                                "name": "name",
                                "type": "string"
                        },
                        {
                                "internalType": "string",
                                "name": "symbol",
                                "type": "string"
                        },
                        {
                                "internalType": "address",
                                "name": "titleEscrowFactory_",
                                "type": "address"
                        }
                ],
                "stateMutability": "nonpayable",
                "type": "constructor"
        },
        {
                "inputs": [],
                "name": "AccessControlBadConfirmation",
                "type": "error"
        },
        {
                "inputs": [
                        {
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        },
                        {
                                "internalType": "bytes32",
                                "name": "neededRole",
                                "type": "bytes32"
                        }
                ],
                "name": "AccessControlUnauthorizedAccount",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "EnforcedPause",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "ExpectedPause",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "InvalidAdminAddress",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "InvalidInitialization",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "InvalidTokenId",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "NotInitializing",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "RemarkLengthExceeded",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "TokenExists",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "TokenNotReturnedToIssuer",
                "type": "error"
        },
        {
                "inputs": [],
                "name": "TransferFailure",
                "type": "error"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": false,
                                "internalType": "uint64",
                                "name": "version",
                                "type": "uint64"
                        }
                ],
                "name": "Initialized",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": false,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        },
                        {
                                "indexed": false,
                                "internalType": "bytes",
                                "name": "remark",
                                "type": "bytes"
                        }
                ],
                "name": "PauseWithRemark",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": false,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        }
                ],
                "name": "Paused",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": true,
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "indexed": true,
                                "internalType": "bytes32",
                                "name": "previousAdminRole",
                                "type": "bytes32"
                        },
                        {
                                "indexed": true,
                                "internalType": "bytes32",
                                "name": "newAdminRole",
                                "type": "bytes32"
                        }
                ],
                "name": "RoleAdminChanged",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": true,
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        },
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "sender",
                                "type": "address"
                        }
                ],
                "name": "RoleGranted",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": true,
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        },
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "sender",
                                "type": "address"
                        }
                ],
                "name": "RoleRevoked",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "from",
                                "type": "address"
                        },
                        {
                                "indexed": true,
                                "internalType": "address",
                                "name": "to",
                                "type": "address"
                        },
                        {
                                "indexed": true,
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        }
                ],
                "name": "Transfer",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": false,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        },
                        {
                                "indexed": false,
                                "internalType": "bytes",
                                "name": "remark",
                                "type": "bytes"
                        }
                ],
                "name": "UnpauseWithRemark",
                "type": "event"
        },
        {
                "anonymous": false,
                "inputs": [
                        {
                                "indexed": false,
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        }
                ],
                "name": "Unpaused",
                "type": "event"
        },
        {
                "inputs": [],
                "name": "ACCEPTER_ROLE",
                "outputs": [
                        {
                                "internalType": "bytes32",
                                "name": "",
                                "type": "bytes32"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "DEFAULT_ADMIN_ROLE",
                "outputs": [
                        {
                                "internalType": "bytes32",
                                "name": "",
                                "type": "bytes32"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "MINTER_ROLE",
                "outputs": [
                        {
                                "internalType": "bytes32",
                                "name": "",
                                "type": "bytes32"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "RESTORER_ROLE",
                "outputs": [
                        {
                                "internalType": "bytes32",
                                "name": "",
                                "type": "bytes32"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "address",
                                "name": "owner",
                                "type": "address"
                        }
                ],
                "name": "balanceOf",
                "outputs": [
                        {
                                "internalType": "uint256",
                                "name": "",
                                "type": "uint256"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        },
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "burn",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "genesis",
                "outputs": [
                        {
                                "internalType": "uint256",
                                "name": "",
                                "type": "uint256"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        }
                ],
                "name": "getRoleAdmin",
                "outputs": [
                        {
                                "internalType": "bytes32",
                                "name": "",
                                "type": "bytes32"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        }
                ],
                "name": "grantRole",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        }
                ],
                "name": "hasRole",
                "outputs": [
                        {
                                "internalType": "bool",
                                "name": "",
                                "type": "bool"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "address",
                                "name": "beneficiary",
                                "type": "address"
                        },
                        {
                                "internalType": "address",
                                "name": "holder",
                                "type": "address"
                        },
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        },
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "mint",
                "outputs": [
                        {
                                "internalType": "address",
                                "name": "",
                                "type": "address"
                        }
                ],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "name",
                "outputs": [
                        {
                                "internalType": "string",
                                "name": "",
                                "type": "string"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "address",
                                "name": "",
                                "type": "address"
                        },
                        {
                                "internalType": "address",
                                "name": "",
                                "type": "address"
                        },
                        {
                                "internalType": "uint256",
                                "name": "",
                                "type": "uint256"
                        },
                        {
                                "internalType": "bytes",
                                "name": "",
                                "type": "bytes"
                        }
                ],
                "name": "onERC721Received",
                "outputs": [
                        {
                                "internalType": "bytes4",
                                "name": "",
                                "type": "bytes4"
                        }
                ],
                "stateMutability": "pure",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        }
                ],
                "name": "ownerOf",
                "outputs": [
                        {
                                "internalType": "address",
                                "name": "",
                                "type": "address"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "pause",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "paused",
                "outputs": [
                        {
                                "internalType": "bool",
                                "name": "",
                                "type": "bool"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "remark",
                "outputs": [
                        {
                                "internalType": "bytes",
                                "name": "",
                                "type": "bytes"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "internalType": "address",
                                "name": "callerConfirmation",
                                "type": "address"
                        }
                ],
                "name": "renounceRole",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        },
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "restore",
                "outputs": [
                        {
                                "internalType": "address",
                                "name": "",
                                "type": "address"
                        }
                ],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "internalType": "address",
                                "name": "account",
                                "type": "address"
                        }
                ],
                "name": "revokeRole",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "string",
                                "name": "baseURI",
                                "type": "string"
                        }
                ],
                "name": "setBaseURI",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes32",
                                "name": "role",
                                "type": "bytes32"
                        },
                        {
                                "internalType": "bytes32",
                                "name": "adminRole",
                                "type": "bytes32"
                        }
                ],
                "name": "setRoleAdmin",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes4",
                                "name": "interfaceId",
                                "type": "bytes4"
                        }
                ],
                "name": "supportsInterface",
                "outputs": [
                        {
                                "internalType": "bool",
                                "name": "",
                                "type": "bool"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "symbol",
                "outputs": [
                        {
                                "internalType": "string",
                                "name": "",
                                "type": "string"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [],
                "name": "titleEscrowFactory",
                "outputs": [
                        {
                                "internalType": "contract ITitleEscrowFactory",
                                "name": "",
                                "type": "address"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        }
                ],
                "name": "tokenURI",
                "outputs": [
                        {
                                "internalType": "string",
                                "name": "",
                                "type": "string"
                        }
                ],
                "stateMutability": "view",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "address",
                                "name": "from",
                                "type": "address"
                        },
                        {
                                "internalType": "address",
                                "name": "to",
                                "type": "address"
                        },
                        {
                                "internalType": "uint256",
                                "name": "tokenId",
                                "type": "uint256"
                        },
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "transferFrom",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        },
        {
                "inputs": [
                        {
                                "internalType": "bytes",
                                "name": "_remark",
                                "type": "bytes"
                        }
                ],
                "name": "unpause",
                "outputs": [],
                "stateMutability": "nonpayable",
                "type": "function"
        }
]`

func TestPack3(t *testing.T) {
	param := "[{\"uint256\": \"123456789111222333\"}]"
	abi, err := ethabi.JSON(strings.NewReader(cJson))
	if err != nil {
		t.Error(err)
	}

	b, err := Pack(abi, "ownerOf", param)
	if err != nil {
		t.Error(err)
	}
	fmt.Printf("%x\n", b)
}

func TestPack4(t *testing.T) {
	a, e := parseUint("uint256", 123456789111222333)
	if e != nil {
		t.Error(e)
	}
	var args []interface{}
	args = append(args, a)
	b := reflect.ValueOf(args[0])
	//fmt.Println(b.Kind() == reflect.Ptr)
	fmt.Println(b.Kind())
}
