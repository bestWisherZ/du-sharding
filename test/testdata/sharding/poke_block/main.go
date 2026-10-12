package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"chainmaker.org/chainmaker/contract-sdk-go/v2/pb/protogo"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sandbox"
	"chainmaker.org/chainmaker/contract-sdk-go/v2/sdk"
)

const (
	keyCounter = "__poke_counter"
	keyLastTx  = "__poke_last_tx"
	keyLastTag = "__poke_last_tag"
)

type PokeBlockContract struct{}

func (c *PokeBlockContract) InitContract() protogo.Response {
	return sdk.Success([]byte("init success"))
}

func (c *PokeBlockContract) UpgradeContract() protogo.Response {
	return sdk.Success([]byte("upgrade success"))
}

func (c *PokeBlockContract) InvokeContract(method string) protogo.Response {
	switch method {
	case "poke":
		return c.Poke()
	case "counter":
		return c.Counter()
	default:
		return sdk.Error("invalid method")
	}
}

// Poke writes state every call so it can always be packed into a block.
func (c *PokeBlockContract) Poke() protogo.Response {
	args := sdk.Instance.GetArgs()
	tag := strings.TrimSpace(string(args["tag"]))

	counter, err := c.loadCounter()
	if err != nil {
		return sdk.Error("load counter failed")
	}
	counter++
	if err := sdk.Instance.PutStateFromKey(keyCounter, strconv.FormatInt(counter, 10)); err != nil {
		return sdk.Error("save counter failed")
	}

	txID, err := sdk.Instance.GetTxId()
	if err != nil {
		return sdk.Error("get tx id failed")
	}
	if err := sdk.Instance.PutStateFromKey(keyLastTx, txID); err != nil {
		return sdk.Error("save last tx failed")
	}
	if err := sdk.Instance.PutStateFromKey(keyLastTag, tag); err != nil {
		return sdk.Error("save last tag failed")
	}

	return sdk.Success([]byte(fmt.Sprintf("poke success, counter=%d", counter)))
}

func (c *PokeBlockContract) Counter() protogo.Response {
	counter, err := c.loadCounter()
	if err != nil {
		return sdk.Error("load counter failed")
	}
	return sdk.Success([]byte(strconv.FormatInt(counter, 10)))
}

func (c *PokeBlockContract) loadCounter() (int64, error) {
	s, err := sdk.Instance.GetStateFromKey(keyCounter)
	if err != nil || strings.TrimSpace(s) == "" {
		return 0, nil
	}
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, err
	}
	return v, nil
}

func main() {
	if err := sandbox.Start(new(PokeBlockContract)); err != nil {
		log.Fatal(err)
	}
}
