package blockchain4

import (
	"encoding/hex"
	"fmt"
	"log"

	"github.com/boltdb/bolt"
)

//dbFile 区块链数据库文件名称
const dbFile = "blockchain.db"
const blocksBucket = "blocks" //存储的内容的键
const genesisCoinbaseData = "The Times 14/Oct/2020 拯救世界，从今天开始。"

//Blockchain 区块链结构
//我们不在里面存储所有的区块了，而是仅存储区块链的 tip。
//另外，我们存储了一个数据库连接。因为我们想要一旦打开它的话，就让它一直运行，直到程序运行结束。
type Blockchain struct {
	Tip []byte   //区块链最后一块的哈希值
	Db  *bolt.DB //数据库
}

//MineBlock 挖出普通区块并将新区块加入到区块链中
//此方法通过区块链的指针调用，将修改区块链bc的内容
func (bc *Blockchain) MineBlock(transactions []*Transaction) {
	var lastHash []byte                         //区块链最后一个区块的哈希
	err := bc.Db.View(func(tx *bolt.Tx) error { //只读打开，读取最后一个区块的哈希，作为新区块的prevHash
		b := tx.Bucket([]byte(blocksBucket))
		lastHash = b.Get([]byte("1")) //最后一个区块的哈希的键是字符串"1"

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	newBlock := NewBlock(transactions, lastHash) //挖出区块

	err = bc.Db.Update(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		err := b.Put(newBlock.Hash, newBlock.Serialize()) //将新区块序列化后插入到数据库表中
		if err != nil {
			log.Panic(err)
		}

		err = b.Put([]byte("1"), newBlock.Hash) //更新区块链最后一个区块的哈希到数据库中
		if err != nil {
			log.Panic(err)
		}

		bc.Tip = newBlock.Hash //修改区块链实例的tip值

		return nil
	})
}

//CreatBlockchain 创建初始区块链
func CreatBlockchain(address string) *Blockchain {
	var tip []byte                          //存储最后一块的哈希
	db, err := bolt.Open(dbFile, 0600, nil) //打开数据库，如果不存在，则创建一个新的
	if err != nil {
		log.Panic(err)
	}

	err = db.Update(func(tx *bolt.Tx) error { //更新数据库，通过事务进行操作。一个数据文件同时只支持一个读-写事务
		b := tx.Bucket([]byte(blocksBucket))

		if b == nil { //不存在数据库表
			fmt.Println("不存在区块链，创建一个新的...")
			//创建区块链的第一个交易：创始交易
			cbtx := NewCoinbaseTX(address, genesisCoinbaseData)
			genesis := NewGenesisBlock(cbtx) //创建创始区块

			b, err := tx.CreateBucket([]byte(blocksBucket)) //创建一个数据库表
			if err != nil {
				log.Panic(err)
			}

			err = b.Put(genesis.Hash, genesis.Serialize()) //键为创始区块的哈希值，值为区块对象序列化后的值，均为二进制数组
			if err != nil {
				log.Panic(err)
			}

			//插入一个键为字符串"1"的二进制数组，值为创始区块的哈希值（保存在数据库表中的区块链的最后一个区块的哈希，其键设置为字符串"1"的二进制数组）
			err = b.Put([]byte("1"), genesis.Hash)
			if err != nil {
				log.Panic(err)
			}
			tip = genesis.Hash //新建区块链后，最后一个区块的哈希
		} else {
			tip = b.Get([]byte("1")) //如果数据库表存在，说明区块链之前已经创建，取出最后一个区块的哈希
		}
		return nil
	})
	if err != nil {
		log.Panic(err)
	}

	BC := Blockchain{tip, db} //构建区块链实例

	return &BC //返回区块链实例的指针
}

//BlockchainIterator 区块链迭代器，用于对区块链中的区块进行迭代
type BlockchainIterator struct {
	currentHash []byte
	db          *bolt.DB
}

//Iterator 每当需要对链中的区块进行迭代时候，我们就通过Blockchain创建迭代器
//注意，迭代器初始状态为链中的tip，因此迭代是从最新到最旧的进行获取
func (bc *Blockchain) Iterator() *BlockchainIterator {
	bci := &BlockchainIterator{bc.Tip, bc.Db}
	return bci
}

//Next 区块链迭代，返回当前区块，并更新迭代器的currentHash为当前区块的PrevBlockHash
func (i *BlockchainIterator) Next() *Block {
	var block *Block

	err := i.db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(blocksBucket))
		encodeBlock := b.Get(i.currentHash)
		block = DeserializeBlock(encodeBlock)

		return nil
	})

	if err != nil {
		log.Panic(err)
	}

	i.currentHash = block.PrevBlockHash

	return block
}

//FindUnspentTransaction 查找未花费的交易（即该交易的花费尚未花出，换句话说，
//及该交易的输出尚未被其他交易作为输入包含进去）
func (bc *Blockchain) FindUnspentTransaction(address string) []Transaction {
	var unspentTXs []Transaction //未花费交易

	//已花费输出，key是转化为字符串的当前交易的ID
	//value是该交易包含的引用输出的所有已花费输出值数组
	//一个交易可能有多个输出，在这种情况下，该交易将引用所有的输出：输出不可分规则，无法引用它的一部分，要么不用，要么一次性用完
	//在go中，映射的值可以是数组，所以这里创建一个映射来存储未花费输出
	spentTXOs := make(map[string][]int)

	bci := bc.Iterator()

	for { //第一层循环，对区块链中的所有区块进行迭代查询
		block := bci.Next()

		for _, tx := range block.Transactions {
			txID := hex.EncodeToString(tx.ID) //交易ID转为字符串，便于比较

		Outputs:
			for outIdx, out := range tx.Vout {
				//检查交易的输出，OutIdx为数组序号，实际上也是某个TxOutput的索引，out为TxOutput
				//一个交易，可能会有多个输出
				//输出是否已经花费了？
				if spentTXOs[txID] != nil {
					for _, spentOut := range spentTXOs[txID] { //spentOut是value
						//根据输出引用不可再分规则，
						//只要有一个输出值被引用，那么该输出的所有值都被引用了
						//所以通过比较索引值，只要发现一个输出值被引用了，就不必查询下一个输出值了
						//说明该输出已经被引用（花费掉了）
						if spentOut == outIdx {
							continue Outputs
						}
					}
				}

				//输出没有花费，且可以用address解锁（归address用户所有）
				if out.CanBeUnlockedWith(address) {
					unspentTXs = append(unspentTXs, *tx) //将tx值加入到已花费交易数组中
				}
			}

			//检查交易的输入，将所有可以解锁的引用的输出加入到已花费输出map中
			//存疑：这段代码似乎应该放到检查输出的代码之前，不然逻辑不合理
			if tx.IsCoinbase() == false {
				for _, in := range tx.Vin {
					if in.CanUnlockOutputWith(address) {
						inTxID := hex.EncodeToString(in.Txid)
						//in.Vout为引用输出在该交易所有输出中的一个索引
						spentTXOs[inTxID] = append(spentTXOs[inTxID], in.Vout)
					}
				}
			}
		}
		if len(block.PrevBlockHash) == 0 { //创始区块都检查完了，退出最外层循环
			break
		}
	}
	return unspentTXs
}
