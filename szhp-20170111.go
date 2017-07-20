/*
Copyright IBM Corp. 2016 All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

		 http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main


//数字汇票

import (
	"errors"
	"fmt"
	"strings"
	"strconv"
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

//数字汇票路径节点信息结构体
type InfoStruct struct {
	Account string 	//账户
	Time string 	//转账截止日期
}

//数字汇票信息结构体
type draftInfoStruct struct {
	Sum string 	//数字汇票金额
	Initiator string 	//发行机构ID
	Target string 	//最终到账机构ID
	Owner string 	//汇票所属机构ID
	PlanPath []InfoStruct 	//计划路径
	TruePath []InfoStruct 	//实际路径
	Status string 	//状态
}

//部署时，传入参数有1个：操作人编号
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting 1")
	}
	return nil, nil
}


func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "transfer" {
		return t.transfer(stub, args)
	}else if function == "create" {
		return t.create(stub, args)
	}else if function == "update" {
		return t.update(stub, args)
	}

	return nil, errors.New("no such a method on this chaincode")
}


//发行数字汇票 参数有3个，一个是数字汇票ID，一个数字汇票信息（json字符串，其中包含的属性有：金额，发行机构ID，最终到账机构ID，当前所属机构ID，计划路径，实际路径），操作人
//数字汇票ID规则设定：九位阿拉伯数字，前八位为大汇票表示，最后一位选择1.2.3,1表示县发行，2表示省发行，3表示ICBC发行。例子：123456781县 123456782省 123456783ICBC
//机构ID：县101+县ID 省20003 ICBC20006 有限合伙20005 SPV102+县ID 项目公司3+xxx
func (t *SimpleChaincode) create(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var draftID string	//数字汇票ID
	var draftInfo string	//数字汇票信息

	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	draftID = args[0]
	draftInfo = args[1]

	// Write the state to the ledger
	err = stub.PutState(draftID, []byte(draftInfo))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//数字汇票所有者转移 传入参数有7个：汇票ID，汇票owner变更由谁变到谁（newOwner），转账金额，转账账户，收款账户，转账时间，操作者编号
//比较的时候比4点，1.金额 2.时间 3.出账账户 4.到账账户 后三个都是在路径中判断的
func (t *SimpleChaincode) transfer(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var draftID string 	//汇票ID
	var newOwnerID string	//汇票变更后汇票所有者ID
	var Sum string	//实际转账金额
	var payAccount string 		//转账账户
	var receiptAccount string 		//收款账户
	var time string 		//转账时间

	var draftInfo draftInfoStruct 	//数字汇票信息结构体
	var tmpDraftInfo draftInfoStruct 	//数字汇票信息临时结构体
	var draftSum string 	//数字汇票金额
	var draftOwner string 	//数字汇票当前所属机构
	var draftPayAccount string 	//数字汇票付款账号
	var draftReceiptAccount string 	//数字汇票收款账号
	var draftPayTime string 	//数字汇票转账时间
	var truePathInfo InfoStruct 	//实际路径该节点的账户和实际转账时间信息结构体
	var draftInfoByte []byte 	//接收汇票信息查询结果
	var totleSum int 	//多张汇票的总金额

	var err error

	if len(args) != 7 {
		return nil, errors.New("Incorrect number of arguments. Expecting 7")
	}

	// Initialize the chaincode
	draftID = args[0]
	newOwnerID = args[1]
	Sum = args[2]
	payAccount = args[3]
	receiptAccount = args[4]
	time = args[5]

	//判断规则：
	//判断点有：1.金额 2.时间 3.出账账户 4.到账账户
	//不需要再判断这张汇票是谁发的，只需要判断这张汇票现阶段属于谁。
	//如果属于20003或者200006或者101xx 则判断金额，出入账账户，出账时间，如果前三个都对只是时间不对，作出标识，但是依然平账，即记录到实际路径中，但是前三个对不上就不能平账
	//如果属于20005，则是对两张会票的操作xxxxxxxx2和xxxxxxxx3.金额为两张的加和，出入账账户、时间两张汇票是一样的，所以选取xxxxxxxx2的信息为准，平账时同时更新两张汇票的实际路径
	//如果属于102xx，则是对两三张会票的操作xxxxxxxx1和xxxxxxxx2和xxxxxxxx3.金额为三张的加和，出入账账户、时间三张汇票是一样的，所以选取xxxxxxxx1的信息为准，平账时同时更新三张汇票的实际路径

	//接收汇票信息查询结果
	draftInfoByte, err = stub.GetState(draftID)
	if err != nil {
		return nil, errors.New("Failed to get state")
	}
	if draftInfoByte == nil {
		return nil, errors.New("Entity not found")
	}
	//将byte的结果转换成struct
	err = json.Unmarshal(draftInfoByte, &draftInfo)  
	if err != nil {  
    	fmt.Println("error:", err)  
	}

	//取出汇票当前所属机构ID
	draftOwner = draftInfo.Owner
	//汇票当前所属机构ID完整ID
	draftOwnerValue, _ := strconv.Atoi(string(draftOwner))
	//汇票当前所属机构ID前三位
	OrganizationIDOfFirstThreeNum := draftOwner[0:3]
	draftOwnerValueOfFirstThreeNum, _ := strconv.Atoi(string(OrganizationIDOfFirstThreeNum))

	//取出该draftID汇票对应的金额
	draftSum = draftInfo.Sum
	//金额是string类型，所以转换成int
	draftSumValue, _ := strconv.Atoi(string(draftSum))

	//ICBC流水信息的金额转换成int类型
	SumValue, _ := strconv.Atoi(string(Sum))

	//判断汇票现所有者是否为20003,20006,101xxx
	if (draftOwnerValueOfFirstThreeNum == 101 || draftOwnerValue == 20003 || draftOwnerValue == 20006) {
		//取出该draftID汇票现阶段对应的出账账户和收款账户和出账时间
		draftPayAccount = draftInfo.PlanPath[0].Account
		draftReceiptAccount = draftInfo.PlanPath[1].Account
		draftPayTime = draftInfo.PlanPath[0].Time
		//判断金额是否相等
		if draftSumValue == SumValue {
			//判断出账账户是否是同一个账户
			if strings.EqualFold(draftPayAccount, payAccount) {
				//判断收款账户是否是同一个账户
				if strings.EqualFold(draftReceiptAccount, receiptAccount) {
					//实际转款时间
					timeValue, _ := strconv.Atoi(string(time)) 
					//计划转款时间
					draftPayTimeValue, _ := strconv.Atoi(string(draftPayTime)) 
					//判断转款时间是否小于计划时间
					//如果转款时间小于计划时间 正常记录到计划路径
					//如果转款时间大于计划时间 在时间后面加上overdue标识
					if timeValue < draftPayTimeValue {
						//汇票需要修改的信息有所属人，实际路径
						truePathInfo.Time = time
					} else {
						//汇票需要修改的信息有所属人，实际路径
						trueTime := time + "-overdue"
						truePathInfo.Time = trueTime
					}
					truePathInfo.Account = payAccount
					//将实际路径节点信息加到汇票信息中去
					draftInfo.TruePath = append(draftInfo.TruePath,truePathInfo)
					//变更汇票所属人
					draftInfo.Owner = newOwnerID
				} else {
					updateStatus(stub,draftID,"The receiptAccount is incorrect!")
					return nil, nil
				}
			} else {
				updateStatus(stub,draftID,"The payAccount is incorrect!")
				return nil, nil
			}
		} else {
			updateStatus(stub,draftID,"The amount of money is incorrect!")
			return nil, nil
		}
	} else if draftOwnerValue == 20005 {
		//上面的if判断汇票现所有者是否为20005
		//取出该draftID汇票现阶段对应的出账账户和收款账户和出账时间
		draftPayAccount = draftInfo.PlanPath[1].Account
		draftReceiptAccount = draftInfo.PlanPath[2].Account
		draftPayTime = draftInfo.PlanPath[1].Time
		//判断金额是否相等 这个金额是xxxxxxxx2和xxxxxxxx3两张汇票金额的加和
		//获取两张张汇票的总金额
		totleSum = 0
		for i := 1; i < 3; i++ {
			//数字汇票ID的最后一位
			lastNumber := strconv.Itoa(i+1)
			//生成数字汇票ID
	  		id := draftID[0 : len(draftID)-1] + lastNumber
	  		//取汇票金额
	  		tmpDraftInfoByte, err := stub.GetState(id)
			if err != nil {
				return nil, errors.New("Failed to get state")
			}
			if tmpDraftInfoByte == nil {
				return nil, errors.New("Entity not found")
			}
			//将byte的结果转换成struct
			err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
			if err != nil {  
		    	fmt.Println("error:", err)  
			}
			//取汇票的金额
			tmpDraftSum := tmpDraftInfo.Sum
			//金额是string类型，所以转换成int
			tmpDraftSumValue, _ := strconv.Atoi(string(tmpDraftSum))
			totleSum = totleSum + tmpDraftSumValue
		}
		//判断金额是否相等
		if totleSum == SumValue {
			//判断出账账户是否是同一个账户
			if strings.EqualFold(draftPayAccount, payAccount) {
				//判断收款账户是否是同一个账户
				if strings.EqualFold(draftReceiptAccount, receiptAccount) {
					//实际转款时间
					timeValue, _ := strconv.Atoi(string(time)) 
					//计划转款时间
					draftPayTimeValue, _ := strconv.Atoi(string(draftPayTime)) 
					//判断转款时间是否小于计划时间
					//如果转款时间小于计划时间 正常记录到计划路径
					//如果转款时间大于计划时间 在时间后面加上overdue标识
					if timeValue < draftPayTimeValue {
						//汇票需要修改的信息有所属人，实际路径
						truePathInfo.Time = time
					} else {
						//汇票需要修改的信息有所属人，实际路径
						trueTime := time + "-overdue"
						truePathInfo.Time = trueTime
					}
					truePathInfo.Account = payAccount
					//将实际路径节点信息加到汇票信息中去
					draftInfo.TruePath = append(draftInfo.TruePath,truePathInfo)
					//变更汇票所属人
					draftInfo.Owner = newOwnerID
					//这一步是要同时变更两张汇票的实际路径，所以，这一步在操作的时候，只传入一张汇票的ID，该汇票的变更在这个函数最下面统一完成，另一张汇票的变更在下面完成
					for i := 1; i < 3; i++ {
						//数字汇票ID的最后一位
						lastNumber := strconv.Itoa(i+1)
						//生成数字汇票ID
				  		id := draftID[0 : len(draftID)-1] + lastNumber
				  		//如果生成的这个汇票id不是draftID，则修改这张汇票的实际路径
				  		if !(strings.EqualFold(id, draftID)) {
				  			
					  		tmpDraftInfoByte, err := stub.GetState(id)
							if err != nil {
								return nil, errors.New("Failed to get state")
							}
							if tmpDraftInfoByte == nil {
								return nil, errors.New("Entity not found")
							}
							//将byte的结果转换成struct
							err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
							if err != nil {  
						    	fmt.Println("error:", err)  
							}
							//将实际路径节点信息加到汇票信息中去
							tmpDraftInfo.TruePath = append(tmpDraftInfo.TruePath,truePathInfo)
							//变更汇票所属人
							tmpDraftInfo.Owner = newOwnerID
							//汇票信息变更完毕，将汇票信息重新存进区块链中
							b, err := json.Marshal(tmpDraftInfo)  
							if err != nil {  
							}

							// Write the state to the ledger
							err = stub.PutState(id, []byte(b))
							if err != nil {
								return nil, err
							}
				  		}

					}
				} else {
					updateStatus(stub,draftID,"The receiptAccount is incorrect!")
					return nil, nil
				}
			} else {
				updateStatus(stub,draftID,"The payAccount is incorrect!")
				return nil, nil
			}
		} else {
			updateStatus(stub,draftID,"The amount of money is incorrect!")
			return nil, nil
		}
	} else if draftOwnerValueOfFirstThreeNum == 102 {
		//上面的if判断汇票现所有者是否为102xx
		//取出该draftID汇票现阶段对应的出账账户和收款账户和出账时间
		//这里的问题在于不同的汇票，spv在其路径中所处的位置是不同的，这里通过对draftID的最后一位数的判断，确定spv在其路径中的位置，即spv在PlanPath这个数组的索引
		//汇票当ID最后一位 
		draftIDOfLastNum := draftID[len(draftID)-1]
		//draftIDOfLastNum为byte类型，转换成it
		draftIDOfLastNumValue, _ := strconv.Atoi(string(draftIDOfLastNum))
		//汇票编号最后一位如果是1，索引设为1，否则设为2
		var index int 
		if draftIDOfLastNumValue == 1 {
			index = 1
		} else {
			index = 2
		}
		draftPayAccount = draftInfo.PlanPath[index].Account
		draftReceiptAccount = draftInfo.PlanPath[index + 1].Account
		draftPayTime = draftInfo.PlanPath[index].Time
		//判断金额是否相等 这个金额是xxxxxxxx1和xxxxxxxx2和xxxxxxxx3三张汇票金额的加和
		//获取三张汇票的总金额
		totleSum = 0
		for i := 0; i < 3; i++ {
			//数字汇票ID的最后一位
			lastNumber := strconv.Itoa(i+1)
			//生成数字汇票ID
	  		id := draftID[0 : len(draftID)-1] + lastNumber
	  		//取汇票金额
	  		tmpDraftInfoByte, err := stub.GetState(id)
			if err != nil {
				return nil, errors.New("Failed to get state")
			}
			if tmpDraftInfoByte == nil {
				return nil, errors.New("Entity not found")
			}
			//将byte的结果转换成struct
			err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
			if err != nil {  
		    	fmt.Println("error:", err)  
			}
			//取汇票的金额
			tmpDraftSum := tmpDraftInfo.Sum
			//金额是string类型，所以转换成int
			tmpDraftSumValue, _ := strconv.Atoi(string(tmpDraftSum))
			totleSum = totleSum + tmpDraftSumValue
		}
		//判断金额是否相等
		if totleSum == SumValue {
			//判断出账账户是否是同一个账户
			if strings.EqualFold(draftPayAccount, payAccount) {
				//判断收款账户是否是同一个账户
				if strings.EqualFold(draftReceiptAccount, receiptAccount) {
					//实际转款时间
					timeValue, _ := strconv.Atoi(string(time)) 
					//计划转款时间
					draftPayTimeValue, _ := strconv.Atoi(string(draftPayTime)) 
					//判断转款时间是否小于计划时间
					//如果转款时间小于计划时间 正常记录到计划路径
					//如果转款时间大于计划时间 在时间后面加上overdue标识
					if timeValue < draftPayTimeValue {
						//汇票需要修改的信息有所属人，实际路径
						truePathInfo.Time = time
					} else {
						//汇票需要修改的信息有所属人，实际路径
						trueTime := time + "-overdue"
						truePathInfo.Time = trueTime
					}
					truePathInfo.Account = payAccount
					//将实际路径节点信息加到汇票信息中去
					draftInfo.TruePath = append(draftInfo.TruePath,truePathInfo)
					//变更汇票所属人
					draftInfo.Owner = newOwnerID
					//这一步是要同时变更两张汇票的实际路径，所以，这一步在操作的时候，只传入一张汇票的ID，该汇票的变更在这个函数最下面统一完成，另一张汇票的变更在下面完成
					for i := 0; i < 3; i++ {
						//数字汇票ID的最后一位
						lastNumber := strconv.Itoa(i+1)
						//生成数字汇票ID
				  		id := draftID[0 : len(draftID)-1] + lastNumber
				  		//如果生成的这个汇票id不是draftID，则修改这张汇票的实际路径
				  		if !(strings.EqualFold(id, draftID)) {
				  			//取汇票金额
					  		tmpDraftInfoByte, err := stub.GetState(id)
							if err != nil {
								return nil, errors.New("Failed to get state")
							}
							if tmpDraftInfoByte == nil {
								return nil, errors.New("Entity not found")
							}
							//将byte的结果转换成struct
							err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
							if err != nil {  
						    	fmt.Println("error:", err)  
							}
							//将实际路径节点信息加到汇票信息中去
							tmpDraftInfo.TruePath = append(tmpDraftInfo.TruePath,truePathInfo)
							//变更汇票所属人
							tmpDraftInfo.Owner = newOwnerID
							//汇票信息变更完毕，将汇票信息重新存进区块链中
							b, err := json.Marshal(tmpDraftInfo)  
							if err != nil {  
							}

							// Write the state to the ledger
							err = stub.PutState(id, []byte(b))
							if err != nil {
								return nil, err
							}
				  		}

					}
				} else {
					updateStatus(stub,draftID,"The receiptAccount is incorrect!")
					return nil, nil
				}
			} else {
				updateStatus(stub,draftID,"The payAccount is incorrect!")
				return nil, nil
			}
		} else {
			updateStatus(stub,draftID,"The amount of money is incorrect!")
			return nil, nil
		}
	} else {
		return nil, errors.New("The draft information is incorrect!")
	}

	//汇票信息变更完毕，将汇票信息重新存进区块链中
	b, err := json.Marshal(draftInfo)  
	if err != nil {  
	}

	// Write the state to the ledger
	err = stub.PutState(draftID, []byte(b))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//平账 参数有3个，一个是数字汇票ID，汇票变更后汇票所属机构ID，操作人
func (t *SimpleChaincode) update(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var draftID string 	//汇票ID
	var newOwnerID string	//汇票变更后汇票所有者ID

	var draftInfoByte []byte 	//接收汇票信息查询结果
	var draftOwner string 	//汇票当前所属机构ID
	var draftInfo draftInfoStruct 	//数字汇票信息结构体
	var tmpDraftInfo draftInfoStruct 	//数字汇票信息临时结构体
	var truePathInfo InfoStruct 	//实际路径该节点的账户和实际转账时间信息结构体

	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	draftID = args[0]
	newOwnerID = args[1]

	//把该汇票该所属机构对应的实际路径按照计划路径填写上去，status更改为""
	//接收汇票信息查询结果
	draftInfoByte, err = stub.GetState(draftID)
	if err != nil {
		return nil, errors.New("Failed to get state")
	}
	if draftInfoByte == nil {
		return nil, errors.New("Entity not found")
	}
	//将byte的结果转换成struct
	err = json.Unmarshal(draftInfoByte, &draftInfo)  
	if err != nil {  
    	fmt.Println("error:", err)  
	}

	//取出汇票当前所属机构ID
	draftOwner = draftInfo.Owner
	//汇票当前所属机构ID完整ID
	draftOwnerValue, _ := strconv.Atoi(string(draftOwner))
	//汇票当前所属机构ID前三位
	OrganizationIDOfFirstThreeNum := draftOwner[0:3]
	draftOwnerValueOfFirstThreeNum, _ := strconv.Atoi(string(OrganizationIDOfFirstThreeNum))

	//判断汇票现所有者是否为20003,20006,101xxx
	if (draftOwnerValueOfFirstThreeNum == 101 || draftOwnerValue == 20003 || draftOwnerValue == 20006) {
		//取出该draftID汇票现阶段对应的出账账户和出账时间
		truePathInfo.Account = draftInfo.PlanPath[0].Account
		truePathInfo.Time = draftInfo.PlanPath[0].Time
	} else if draftOwnerValue == 20005 {
		//上面的if判断汇票现所有者是否为20005
		//取出该draftID汇票现阶段对应的出账账户和收款账户和出账时间
		truePathInfo.Account = draftInfo.PlanPath[1].Account
		truePathInfo.Time = draftInfo.PlanPath[1].Time
		
		//这一步是要同时变更两张汇票的实际路径，所以，这一步在操作的时候，只传入一张汇票的ID，该汇票的变更在这个函数最下面统一完成，另一张汇票的变更在下面完成
		for i := 1; i < 3; i++ {
			//数字汇票ID的最后一位
			lastNumber := strconv.Itoa(i+1)
			//生成数字汇票ID
			id := draftID[0 : len(draftID)-1] + lastNumber
			//如果生成的这个汇票id不是draftID，则修改这张汇票的实际路径
			if !(strings.EqualFold(id, draftID)) {
				//取汇票
				tmpDraftInfoByte, err := stub.GetState(id)
				if err != nil {
					return nil, errors.New("Failed to get state")
				}
				if tmpDraftInfoByte == nil {
					return nil, errors.New("Entity not found")
				}
				//将byte的结果转换成struct
				err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
				if err != nil {  
					fmt.Println("error:", err)  
				}
				truePathInfo.Account = tmpDraftInfo.PlanPath[1].Account
				truePathInfo.Time = tmpDraftInfo.PlanPath[1].Time


				//将实际路径节点信息加到汇票信息中去
				tmpDraftInfo.TruePath = append(tmpDraftInfo.TruePath,truePathInfo)
				//变更汇票所属人
				tmpDraftInfo.Owner = newOwnerID
				//汇票信息变更完毕，将汇票信息重新存进区块链中
				b, err := json.Marshal(tmpDraftInfo)  
				if err != nil {  
				}

				// Write the state to the ledger
				err = stub.PutState(id, []byte(b))
				if err != nil {
					return nil, err
				}
			}
		}
	} else if draftOwnerValueOfFirstThreeNum == 102 {
		//上面的if判断汇票现所有者是否为102xx
		//取出该draftID汇票现阶段对应的出账账户和收款账户和出账时间
		//这里的问题在于不同的汇票，spv在其路径中所处的位置是不同的，这里通过对draftID的最后一位数的判断，确定spv在其路径中的位置，即spv在PlanPath这个数组的索引
		//汇票当ID最后一位 
		draftIDOfLastNum := draftID[len(draftID)-1]
		//draftIDOfLastNum为byte类型，转换成it
		draftIDOfLastNumValue, _ := strconv.Atoi(string(draftIDOfLastNum))
		//汇票编号最后一位如果是1，索引设为1，否则设为2
		var index int 
		if draftIDOfLastNumValue == 1 {
			index = 1
		} else {
			index = 2
		}
		truePathInfo.Account = draftInfo.PlanPath[index].Account
		truePathInfo.Time = draftInfo.PlanPath[index].Time
		//这一步是要同时变更两张汇票的实际路径，所以，这一步在操作的时候，只传入一张汇票的ID，该汇票的变更在这个函数最下面统一完成，另一张汇票的变更在下面完成
		for i := 0; i < 3; i++ {
			//数字汇票ID的最后一位
			lastNumber := strconv.Itoa(i+1)
			//生成数字汇票ID
			id := draftID[0 : len(draftID)-1] + lastNumber
			//如果生成的这个汇票id不是draftID，则修改这张汇票的实际路径
			if !(strings.EqualFold(id, draftID)) {
				//取汇票
				tmpDraftInfoByte, err := stub.GetState(id)
				if err != nil {
					return nil, errors.New("Failed to get state")
				}
				if tmpDraftInfoByte == nil {
					return nil, errors.New("Entity not found")
				}
				//将byte的结果转换成struct
				err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
				if err != nil {  
					fmt.Println("error:", err)  
				}
				truePathInfo.Account = tmpDraftInfo.PlanPath[index].Account
				truePathInfo.Time = tmpDraftInfo.PlanPath[index].Time


				//将实际路径节点信息加到汇票信息中去
				tmpDraftInfo.TruePath = append(tmpDraftInfo.TruePath,truePathInfo)
				//变更汇票所属人
				tmpDraftInfo.Owner = newOwnerID
				//汇票信息变更完毕，将汇票信息重新存进区块链中
				b, err := json.Marshal(tmpDraftInfo)  
				if err != nil {  
				}

				// Write the state to the ledger
				err = stub.PutState(id, []byte(b))
				if err != nil {
					return nil, err
				}
			}
		}
	} else {
		return nil, errors.New("The draft information is incorrect!")
	}

	//将实际路径节点信息加到汇票信息中去
	draftInfo.TruePath = append(draftInfo.TruePath,truePathInfo)

	draftInfo.Status = ""

	//汇票信息变更完毕，将汇票信息重新存进区块链中
	b, err := json.Marshal(draftInfo)  
	if err != nil {  
	}

	// Write the state to the ledger
	err = stub.PutState(draftID, []byte(b))
	if err != nil {
		return nil, err
	}
	return nil, nil
}

func updateStatus(stub shim.ChaincodeStubInterface, draftID string, statusInfo string) (error){
	var info string 	//status的信息
	var ID string 	//汇票ID
	var tmpDraftInfo draftInfoStruct 	//数字汇票信息临时结构体
	var err error

	info = statusInfo
	ID = draftID

	tmpDraftInfoByte, err := stub.GetState(ID)
	if err != nil {
		return errors.New("Failed to get state")
	}
	if tmpDraftInfoByte == nil {
		return errors.New("Entity not found")
	}			
	//将byte的结果转换成struct
	err = json.Unmarshal(tmpDraftInfoByte, &tmpDraftInfo)  
	if err != nil {  
		fmt.Println("error:", err)  
	}
	tmpDraftInfo.Status = info
				
	//汇票信息变更完毕，将汇票信息重新存进区块链中
	b, err := json.Marshal(tmpDraftInfo)  
	if err != nil {  
	}

	// Write the state to the ledger
	err = stub.PutState(draftID, []byte(b))
	if err != nil {
		return err
	}
	return nil
}


// Query callback representing the query of a chaincode
func (t *SimpleChaincode) Query(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function != "query" {
		return nil, errors.New("Invalid query function name. Expecting \"query\"")
	}
	var A string // Entities
	var err error

	if len(args) != 1 {
		return nil, errors.New("Incorrect number of arguments. Expecting name of the person to query")
	}

	A = args[0]

	// Get the state from the ledger
	Avalbytes, err := stub.GetState(A)
	if err != nil {
		jsonResp := "{\"Error\":\"Failed to get state for " + A + "\"}"
		return nil, errors.New(jsonResp)
	}

	if Avalbytes == nil {
		jsonResp := "{\"Error\":\"Nil amount for " + A + "\"}"
		return nil, errors.New(jsonResp)
	}

	jsonResp := "{\"Name\":\"" + A + "\",\"Amount\":\"" + string(Avalbytes) + "\"}"
	fmt.Printf("Query Response:%s\n", jsonResp)
	return Avalbytes, nil
}

func main() {
	err := shim.Start(new(SimpleChaincode))
	if err != nil {
		fmt.Printf("Error starting Simple chaincode: %s", err)
	}
}
