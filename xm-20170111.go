
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


//项目

import (
	"errors"
	"fmt"
	"strconv"
	"encoding/json"

	"github.com/hyperledger/fabric/core/chaincode/shim"
)

// SimpleChaincode example simple Chaincode implementation
type SimpleChaincode struct {
}

//审核结果结构体 Office表示指挥部办公室 Government表示县政府
type ApprovalStruct struct {
	Office string
	Government string
}

//数字汇票结构体
type DraftStruct struct {
	DraftID string
	DraftMount string
}

//资金进度结构体
type FundStruct struct {
	Priority1 DraftStruct
	Priority2 DraftStruct
	Priority3 DraftStruct
}



//部署时，传入参数有3个 项目ID，项目信息，操作人ID
//变量名ProjectHash解释，这个里面有个hash，不要理解错了，这是因为原来设计的时候是要存项目信息的hash，而现在的设计是要存项目全信息
func (t *SimpleChaincode) Init(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {

	var ProjectID string	//项目ID
	var ProjectHash string	//项目信息
	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	ProjectID = args[0]
	ProjectHash = args[1]

	// Write the state to the ledger
	err = stub.PutState("ProjectID", []byte(ProjectID))
	if err != nil {
		return nil, err
	}
	err = stub.PutState("ProjectHash", []byte(ProjectHash))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

func (t *SimpleChaincode) Invoke(stub shim.ChaincodeStubInterface, function string, args []string) ([]byte, error) {
	if function == "updateApproval" {
		return t.updateApproval(stub, args)
	}else if function == "updateProject"{
		return t.updateProject(stub,args)
	}else if function == "updateProjectProgress"{
		return t.updateProjectProgress(stub,args)
	}else if function == "updateFundProgress"{
		return t.updateFundProgress(stub,args)
	}

	return nil, errors.New("no such a method on this chaincode")
}

//审核项目 传入参数有3个：项目审核进度（审核机构编号（202+县ID表示指挥部办公室，103+县ID表示县政府），审核结果），操作人编号
func (t *SimpleChaincode) updateApproval(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	
	var OrganizationID string	//审核机构编号
	var OrganizationResult string 	//该审核机构审核结果
	var ApprovalResult []byte 	//审核结果
	var ID int 	//审核机构的编号 int类型
	var ResultStruct ApprovalStruct 	//审核结果结构体
	//var TmpStruct ApprovalStruct 	//将查询结果解析成结构体
	var TmpResult []byte 	//用于存放查询结果
	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	OrganizationID = args[0]
	OrganizationResult = args[1]

	//接收查询结果
	TmpResult, _ = stub.GetState("ApprovalResult")
	//判断审查结果的值，如果为空，说明这是第一次录入结果，给ResultStruct赋空值
	if TmpResult == nil {
		ResultStruct.Office = ""
		ResultStruct.Government = ""
	}else{
		//如果不为空，说明之前已经有审查结果了，将之前的值赋给ResultStruct
		err := json.Unmarshal(TmpResult, &ResultStruct)
		if err != nil {  
    	fmt.Println("error:", err)  
		}
	}

	//取出机构ID的前三位 
	OrganizationIDOfFirstThreeNum := OrganizationID[0:3]
	ID, _ = strconv.Atoi(OrganizationIDOfFirstThreeNum)

	//赋新值
	if ID == 202 {
		ResultStruct.Office = OrganizationResult
	}else if ID == 103 {
		ResultStruct.Government = OrganizationResult
	}

	//将struct转移成json []byte格式
	ApprovalResult,_ = json.Marshal(ResultStruct)

	// Write the state to the ledger
	err = stub.PutState("ApprovalResult", ApprovalResult)
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//修改项目 传入参数有3个：项目编号，项目信息，操作者编号
func (t *SimpleChaincode) updateProject(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var NewProjectID string	//项目ID
	var NewProjectHash string	//项目信息hash
	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	NewProjectID = args[0]
	NewProjectHash = args[1]

	// Write the state to the ledger
	err = stub.PutState("ProjectID", []byte(NewProjectID))
	if err != nil {
		return nil, err
	}
	err = stub.PutState("ProjectHash", []byte(NewProjectHash))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//项目进度 传入参数有3个：项目进度（百分数），项目进度说明，操作者编号
func (t *SimpleChaincode) updateProjectProgress(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var ProjectProgress string	//项目进度
	var ProjectProgressExplain string	//项目进度说明
	var err error

	if len(args) != 3 {
		return nil, errors.New("Incorrect number of arguments. Expecting 3")
	}

	// Initialize the chaincode
	ProjectProgress = args[0]
	ProjectProgressExplain = args[1]

	// Write the state to the ledger
	err = stub.PutState("ProjectProgress", []byte(ProjectProgress))
	if err != nil {
		return nil, err
	}
	err = stub.PutState("ProjectProgressExplain", []byte(ProjectProgressExplain))
	if err != nil {
		return nil, err
	}

	return nil, nil
}

//资金进度 传入参数有4个：资金进度（汇票发行机构（101+县ID,20003,20006），数字汇票编号,金额），操作者编号
func (t *SimpleChaincode) updateFundProgress(stub shim.ChaincodeStubInterface, args []string) ([]byte, error) {
	var OrganizationID string	//汇票发行机构
	var DraftID string 	//数字汇票编号
	var DraftMount string 	//数字汇票金额
	var FundProgress []byte 	//资金进度
	var TmpResult []byte 	//用于存放查询结果
	var ResultStruct FundStruct 	//查询结果结构体 
	var ID int 	//汇票发行机构的编号 int类型

	var err error

	if len(args) != 4 {
		return nil, errors.New("Incorrect number of arguments. Expecting 4")
	}

	// Initialize the chaincode
	OrganizationID = args[0]
	DraftID = args[1]
	DraftMount = args[2]

	//接收查询结果
	TmpResult, _ = stub.GetState("FundProgress")
	//判断查询结果，如果为空，说明这是第一次录入结果，给ResultStruct赋空值
	if TmpResult == nil {
		ResultStruct.Priority1.DraftID = ""
		ResultStruct.Priority1.DraftMount = ""
		ResultStruct.Priority2.DraftID = ""
		ResultStruct.Priority2.DraftMount = ""
		ResultStruct.Priority3.DraftID = ""
		ResultStruct.Priority3.DraftMount = ""
	}else{
		//如果不为空，说明之前已经有审查结果了，将之前的值赋给ResultStruct
		err := json.Unmarshal(TmpResult, &ResultStruct)
		if err != nil {  
    	fmt.Println("error:", err)  
		}
	}

	//根据传入参数 赋新值
	ID, _ = strconv.Atoi(OrganizationID)

	if ID == 20003 {
		ResultStruct.Priority2.DraftID = DraftID
		ResultStruct.Priority2.DraftMount = DraftMount
	}else if ID == 20006 {
		ResultStruct.Priority3.DraftID = DraftID
		ResultStruct.Priority3.DraftMount = DraftMount
	}else {
		//取出机构ID的前三位 
		OrganizationIDOfFirstThreeNum := OrganizationID[0:3]
		ID, _ = strconv.Atoi(OrganizationIDOfFirstThreeNum)
		if ID == 101 {
			ResultStruct.Priority1.DraftID = DraftID
			ResultStruct.Priority1.DraftMount = DraftMount
		}else {
			return nil, errors.New("OrganizationID is incorrectly") 
		}
	}

	//将struct转移成json []byte格式
	FundProgress,_ = json.Marshal(ResultStruct)

	// Write the state to the ledger
	err = stub.PutState("FundProgress", []byte(FundProgress))
	if err != nil {
		return nil, err
	}

	return nil, nil
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
