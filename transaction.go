package blockchain4

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"fmt"
	"log"
)

const subsidy = 10 //挖矿奖励

//Transaction 交易结构，代表一个交易
type Transaction struct {
	ID   []byte     //交易ID
	Vin  []TxInput  //交易输入，由上次交易输入（可能多个）
	Vout []TxOutput //交易输出，由本次交易产生（可能多个）
}

//IsCoinbase 检查交易是否是创始区块交易
//创始区块交易没有输入，详细见NewCoinbaseTX
//tx.Vin只有一个输入，数组长度为1
//tx.Vin[0].Txid为[]byte{}，因此长度为0
//Vin[0].Vout设置为-1
func (tx Transaction) IsCoinbase() bool {
	return len(tx.Vin) == 1 && len(tx.Vin[0].Txid) == 0 && tx.Vin[0].Vout == -1
}

//TxInput 交易的输入
//包含的是前一笔交易的一个输出
type TxInput struct {
	Txid []byte //前一笔交易的ID
	Vout int    //前一笔交易在该笔交易所有输出中的索引（一笔交易可能有多个输出，需要有信息指明具体是哪一个）

	//一个脚本，可作用于一个输出的scriptPubKey的数据，用于解锁输出
	//如果ScriptSig是正确的，那么引用的输出就会被解锁，然后被解锁的值就可以被用于产生新的输出
	//如果不正确，前一笔交易的输出就无法被引用在输入中，或者说，也就无法使用这个输出
	//这种机制，保证了用户无法花费其他人的币
	//这里仅仅存储用户的钱包地址
	ScriptSig string
}

//TxOutput 交易的输出
type TxOutput struct {
	value int //输出里面存储的“币”

	//解锁脚本（比特币里面是一个脚本，这里是用户的钱包地址），定义了
	//解锁该输出的逻辑。
	ScriptPubKey string
}

//SetID 设置交易的ID：计算出交易实例的哈希，作为交易的ID
//注意，ID是一个[32]byte数组
func (tx *Transaction) SetID() {
	var encoded bytes.Buffer
	var hash [32]byte //32字节，256位

	enc := gob.NewEncoder(&encoded)
	err := enc.Encode(tx) //编码为byte.Buffer类型
	if err != nil {
		log.Panic(err)
	}
	hash = sha256.Sum256(encoded.Bytes())
	tx.ID = hash[:]
}

//CanUnlockOutputWith 检查是否是地址unlockingData发起的交易
//进行发起者unlockingData的身份检查
//此方法的作用存疑
func (in *TxInput) CanUnlockOutputWith(unlockingData string) bool {
	return in.ScriptSig == unlockingData
}

//CanBeUnlockedWith 检查输出是否可以被提供的数据unlockingData解锁
//检查解锁数据的正确性，正确则返回true，否则返回false
func (out *TxOutput) CanBeUnlockedWith(unlockingData string) bool {
	return out.ScriptPubKey == unlockingData
}

//NewCoinbaseTX 创建一个区块链创始交易
func NewCoinbaseTX(to, data string) *Transaction {
	if data == "" {
		data = fmt.Sprintf("Reward to %s", to) //fmt.Sprintf将数据格式化后赋值给变量data
	}

	//初始交易输入结构：引用输出的交易为空:引用交易的ID为空，交易输出值为设为-1
	txin := TxInput{[]byte{}, -1, data}
	txout := TxOutput{subsidy, to}                             //本次交易的输出结构：奖励值为subsidy，奖励给地址to（当然也只有地址to可以解锁使用这笔钱）
	tx := Transaction{nil, []TxInput{txin}, []TxOutput{txout}} //交易ID设为nil

	return &tx
}
