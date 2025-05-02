package crdtpatch

import (
	"encoding/json"
	"time"

	"tictactoe/luvjson/common"
	"tictactoe/luvjson/crdt"
)

// Operation represents a JSON CRDT Patch operation.
type Operation interface {
	// Type returns the type of the operation.
	Type() common.OperationType

	// GetID returns the ID of the operation.
	GetID() common.LogicalTimestamp

	// Apply applies the operation to the document.
	Apply(doc *crdt.Document) error

	// Span returns the number of logical clock cycles the operation takes.
	Span() uint64

	// MarshalJSON returns a JSON representation of the operation.
	json.Marshaler

	// UnmarshalJSON parses a JSON representation of the operation.
	json.Unmarshaler
}

// MakeOperation creates a new operation based on the operation type.
func MakeOperation(opType common.OperationType, id common.LogicalTimestamp) Operation {
	switch opType {
	case common.OperationTypeNew:
		return &NewOperation{ID: id}
	case common.OperationTypeIns:
		return &InsOperation{ID: id}
	case common.OperationTypeDel:
		return &DelOperation{ID: id}
	case common.OperationTypeNop:
		return &NopOperation{ID: id}
	default:
		return nil
	}
}

// NewOperation represents an operation that creates a new CRDT node.
type NewOperation struct {
	ID       common.LogicalTimestamp
	NodeType common.NodeType
	Value    interface{}
}

// Type returns the type of the operation.
func (o *NewOperation) Type() common.OperationType {
	return common.OperationTypeNew
}

// GetID returns the ID of the operation.
func (o *NewOperation) GetID() common.LogicalTimestamp {
	return o.ID
}

// Apply applies the operation to the document.
func (o *NewOperation) Apply(doc *crdt.Document) error {
	var node crdt.Node

	switch o.NodeType {
	case common.NodeTypeCon:
		// 시간 값을 처리하기 위한 특별한 로직
		if timeVal, ok := o.Value.(map[string]interface{}); ok {
			if timeType, ok := timeVal["type"].(string); ok && timeType == "time" {
				if timeStr, ok := timeVal["value"].(string); ok {
					// RFC3339 형식의 시간 문자열을 time.Time으로 파싱
					parsedTime, err := time.Parse(time.RFC3339, timeStr)
					if err == nil {
						// 파싱된 시간을 사용
						node = crdt.NewConstantNode(o.ID, parsedTime)
						break
					}
				}
			}
		}
		node = crdt.NewConstantNode(o.ID, o.Value)
	case common.NodeTypeVal:
		node = crdt.NewLWWValueNode(o.ID, o.ID, crdt.NewConstantNode(o.ID, nil))
	case common.NodeTypeObj:
		node = crdt.NewLWWObjectNode(o.ID)
	case common.NodeTypeStr:
		node = crdt.NewRGAStringNode(o.ID)
	case common.NodeTypeRoot:
		node = crdt.NewRootNode(o.ID)
	default:
		// 지원되지 않는 노드 타입에 대한 오류 메시지 개선
		return common.ErrInvalidNodeType{Type: string(o.NodeType)}
	}

	doc.AddNode(node)
	return nil
}

// Span returns the number of logical clock cycles the operation takes.
func (o *NewOperation) Span() uint64 {
	return 1
}

// MarshalJSON returns a JSON representation of the operation.
func (o *NewOperation) MarshalJSON() ([]byte, error) {
	type jsonOp struct {
		Op    string                  `json:"op"`
		ID    common.LogicalTimestamp `json:"id"`
		Type  string                  `json:"type,omitempty"`
		Value interface{}             `json:"value,omitempty"`
	}

	op := jsonOp{
		Op: "new_" + string(o.NodeType),
		ID: o.ID,
	}

	if o.NodeType == common.NodeTypeCon {
		// time.Time 값을 특별하게 처리
		if timeVal, ok := o.Value.(time.Time); ok {
			// time.Time 값을 RFC3339 형식의 문자열로 변환하여 저장
			op.Value = map[string]interface{}{
				"type":  "time",
				"value": timeVal.Format(time.RFC3339),
			}
		} else {
			op.Value = o.Value
		}
	}

	return json.Marshal(op)
}

// UnmarshalJSON parses a JSON representation of the operation.
func (o *NewOperation) UnmarshalJSON(data []byte) error {
	var op map[string]interface{}
	if err := json.Unmarshal(data, &op); err != nil {
		return err
	}

	opStr, ok := op["op"].(string)
	if !ok {
		return common.ErrInvalidOperation{Message: "missing or invalid 'op' field"}
	}

	if len(opStr) < 4 || opStr[:4] != "new_" {
		return common.ErrInvalidOperation{Message: "not a 'new' operation"}
	}

	// Parse the ID field
	idJSON, ok := op["id"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'id' field"}
	}

	// Get the ID in the new format (object with sid and cnt)
	idMap, ok := idJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "id must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	sidVal, sidOk := idMap["sid"]
	cntVal, cntOk := idMap["cnt"]

	if !sidOk || !cntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'id'"}
	}

	// Marshal the SID value to JSON
	sidJSON, err := json.Marshal(sidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var sid common.SessionID
	if err := sid.UnmarshalJSON(sidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Get the counter value
	var counter uint64
	switch v := cntVal.(type) {
	case float64:
		counter = uint64(v)
	case int:
		counter = uint64(v)
	case int64:
		counter = uint64(v)
	case uint64:
		counter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number"}
	}

	// Set the operation ID
	o.ID = common.LogicalTimestamp{SID: sid, Counter: counter}

	o.NodeType = common.NodeType(opStr[4:])

	if o.NodeType == common.NodeTypeCon {
		o.Value = op["value"]
	}

	return nil
}

// InsOperation represents an operation that updates an existing CRDT node.
type InsOperation struct {
	ID       common.LogicalTimestamp
	TargetID common.LogicalTimestamp
	Value    interface{}
}

// Type returns the type of the operation.
func (o *InsOperation) Type() common.OperationType {
	return common.OperationTypeIns
}

// GetID returns the ID of the operation.
func (o *InsOperation) GetID() common.LogicalTimestamp {
	return o.ID
}

// Apply applies the operation to the document.
func (o *InsOperation) Apply(doc *crdt.Document) error {
	target, err := doc.GetNode(o.TargetID)
	if err != nil {
		return err
	}

	switch node := target.(type) {
	case *crdt.LWWValueNode:
		// Update the value
		// time.Time 값을 특별하게 처리
		if timeVal, ok := o.Value.(map[string]interface{}); ok {
			if timeType, ok := timeVal["type"].(string); ok && timeType == "time" {
				if timeStr, ok := timeVal["value"].(string); ok {
					// RFC3339 형식의 시간 문자열을 time.Time으로 파싱
					parsedTime, err := time.Parse(time.RFC3339, timeStr)
					if err == nil {
						// 파싱된 시간을 사용
						valueNode := crdt.NewConstantNode(o.ID, parsedTime)
						node.SetValue(o.ID, valueNode)
						doc.AddNode(valueNode)
						return nil
					}
				}
			}
		}
		valueNode := crdt.NewConstantNode(o.ID, o.Value)
		node.SetValue(o.ID, valueNode)
		doc.AddNode(valueNode)
	case *crdt.LWWObjectNode:
		// Update a field
		if obj, ok := o.Value.(map[string]interface{}); ok {
			for key, val := range obj {
				// time.Time 값을 특별하게 처리
				if timeVal, ok := val.(map[string]interface{}); ok {
					if timeType, ok := timeVal["type"].(string); ok && timeType == "time" {
						if timeStr, ok := timeVal["value"].(string); ok {
							// RFC3339 형식의 시간 문자열을 time.Time으로 파싱
							parsedTime, err := time.Parse(time.RFC3339, timeStr)
							if err == nil {
								// 파싱된 시간을 사용
								valueNode := crdt.NewConstantNode(o.ID, parsedTime)
								node.Set(key, o.ID, valueNode)
								doc.AddNode(valueNode)
								continue
							}
						}
					}
				}
				valueNode := crdt.NewConstantNode(o.ID, val)
				node.Set(key, o.ID, valueNode)
				doc.AddNode(valueNode)
			}
		}
	case *crdt.RGAStringNode:
		// Insert a string
		if str, ok := o.Value.(string); ok {
			node.Insert(o.TargetID, o.ID, str)
		}
	// Add other node types as needed
	default:
		return common.ErrInvalidOperation{Message: "unsupported node type for 'ins' operation"}
	}

	return nil
}

// Span returns the number of logical clock cycles the operation takes.
func (o *InsOperation) Span() uint64 {
	return 1
}

// MarshalJSON returns a JSON representation of the operation.
func (o *InsOperation) MarshalJSON() ([]byte, error) {
	type jsonOp struct {
		Op    string                  `json:"op"`
		ID    common.LogicalTimestamp `json:"id"`
		Obj   common.LogicalTimestamp `json:"obj"`
		Value interface{}             `json:"value,omitempty"`
	}

	op := jsonOp{
		Op:  "ins",
		ID:  o.ID,
		Obj: o.TargetID,
	}

	// time.Time 값을 특별하게 처리
	if timeVal, ok := o.Value.(time.Time); ok {
		// time.Time 값을 RFC3339 형식의 문자열로 변환하여 저장
		op.Value = map[string]interface{}{
			"type":  "time",
			"value": timeVal.Format(time.RFC3339),
		}
	} else if mapVal, ok := o.Value.(map[string]interface{}); ok {
		// 맵 내부의 time.Time 값을 처리
		processedMap := make(map[string]interface{})
		for k, v := range mapVal {
			if timeVal, ok := v.(time.Time); ok {
				processedMap[k] = map[string]interface{}{
					"type":  "time",
					"value": timeVal.Format(time.RFC3339),
				}
			} else {
				processedMap[k] = v
			}
		}
		op.Value = processedMap
	} else {
		op.Value = o.Value
	}

	return json.Marshal(op)
}

// UnmarshalJSON parses a JSON representation of the operation.
func (o *InsOperation) UnmarshalJSON(data []byte) error {
	var op map[string]interface{}
	if err := json.Unmarshal(data, &op); err != nil {
		return err
	}

	opStr, ok := op["op"].(string)
	if !ok || opStr != "ins" {
		return common.ErrInvalidOperation{Message: "not an 'ins' operation"}
	}

	// Parse the ID field
	idJSON, ok := op["id"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'id' field"}
	}

	// Get the ID in the new format (object with sid and cnt)
	idMap, ok := idJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "id must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	sidVal, sidOk := idMap["sid"]
	cntVal, cntOk := idMap["cnt"]

	if !sidOk || !cntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'id'"}
	}

	// Marshal the SID value to JSON
	sidJSON, err := json.Marshal(sidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var sid common.SessionID
	if err := sid.UnmarshalJSON(sidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Get the counter value
	var counter uint64
	switch v := cntVal.(type) {
	case float64:
		counter = uint64(v)
	case int:
		counter = uint64(v)
	case int64:
		counter = uint64(v)
	case uint64:
		counter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number"}
	}

	// Set the operation ID
	o.ID = common.LogicalTimestamp{SID: sid, Counter: counter}

	// Parse the obj field
	objJSON, ok := op["obj"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'obj' field"}
	}

	// Get the obj in the new format (object with sid and cnt)
	objMap, ok := objJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "obj must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	objSidVal, objSidOk := objMap["sid"]
	objCntVal, objCntOk := objMap["cnt"]

	if !objSidOk || !objCntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'obj'"}
	}

	// Marshal the SID value to JSON
	objSidJSON, err := json.Marshal(objSidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field in obj"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var objSid common.SessionID
	if err := objSid.UnmarshalJSON(objSidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field in obj"}
	}

	// Get the counter value
	var objCounter uint64
	switch v := objCntVal.(type) {
	case float64:
		objCounter = uint64(v)
	case int:
		objCounter = uint64(v)
	case int64:
		objCounter = uint64(v)
	case uint64:
		objCounter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number in obj"}
	}

	// Set the target ID
	o.TargetID = common.LogicalTimestamp{SID: objSid, Counter: objCounter}

	o.Value = op["value"]

	return nil
}

// DelOperation represents an operation that deletes contents from an existing CRDT node.
type DelOperation struct {
	ID       common.LogicalTimestamp
	TargetID common.LogicalTimestamp
	Key      string
	StartID  common.LogicalTimestamp
	EndID    common.LogicalTimestamp
}

// Type returns the type of the operation.
func (o *DelOperation) Type() common.OperationType {
	return common.OperationTypeDel
}

// GetID returns the ID of the operation.
func (o *DelOperation) GetID() common.LogicalTimestamp {
	return o.ID
}

// Apply applies the operation to the document.
func (o *DelOperation) Apply(doc *crdt.Document) error {
	target, err := doc.GetNode(o.TargetID)
	if err != nil {
		return err
	}

	switch node := target.(type) {
	case *crdt.LWWObjectNode:
		// Delete a field
		node.Delete(o.Key, o.ID)
	case *crdt.RGAStringNode:
		// Delete a range of characters
		node.Delete(o.StartID, o.EndID)
	// Add other node types as needed
	default:
		return common.ErrInvalidOperation{Message: "unsupported node type for 'del' operation"}
	}

	return nil
}

// Span returns the number of logical clock cycles the operation takes.
func (o *DelOperation) Span() uint64 {
	return 1
}

// MarshalJSON returns a JSON representation of the operation.
func (o *DelOperation) MarshalJSON() ([]byte, error) {
	type jsonOp struct {
		Op    string                  `json:"op"`
		ID    common.LogicalTimestamp `json:"id"`
		Obj   common.LogicalTimestamp `json:"obj"`
		Key   string                  `json:"key,omitempty"`
		Start common.LogicalTimestamp `json:"start,omitempty"`
		End   common.LogicalTimestamp `json:"end,omitempty"`
	}

	op := jsonOp{
		Op:  "del",
		ID:  o.ID,
		Obj: o.TargetID,
	}

	if o.Key != "" {
		op.Key = o.Key
	} else {
		op.Start = o.StartID
		op.End = o.EndID
	}

	return json.Marshal(op)
}

// UnmarshalJSON parses a JSON representation of the operation.
func (o *DelOperation) UnmarshalJSON(data []byte) error {
	var op map[string]interface{}
	if err := json.Unmarshal(data, &op); err != nil {
		return err
	}

	opStr, ok := op["op"].(string)
	if !ok || opStr != "del" {
		return common.ErrInvalidOperation{Message: "not a 'del' operation"}
	}

	// Parse the ID field
	idJSON, ok := op["id"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'id' field"}
	}

	// Get the ID in the new format (object with sid and cnt)
	idMap, ok := idJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "id must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	sidVal, sidOk := idMap["sid"]
	cntVal, cntOk := idMap["cnt"]

	if !sidOk || !cntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'id'"}
	}

	// Marshal the SID value to JSON
	sidJSON, err := json.Marshal(sidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var sid common.SessionID
	if err := sid.UnmarshalJSON(sidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Get the counter value
	var counter uint64
	switch v := cntVal.(type) {
	case float64:
		counter = uint64(v)
	case int:
		counter = uint64(v)
	case int64:
		counter = uint64(v)
	case uint64:
		counter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number"}
	}

	// Set the operation ID
	o.ID = common.LogicalTimestamp{SID: sid, Counter: counter}

	// Parse the obj field
	objJSON, ok := op["obj"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'obj' field"}
	}

	// Get the obj in the new format (object with sid and cnt)
	objMap, ok := objJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "obj must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	objSidVal, objSidOk := objMap["sid"]
	objCntVal, objCntOk := objMap["cnt"]

	if !objSidOk || !objCntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'obj'"}
	}

	// Marshal the SID value to JSON
	objSidJSON, err := json.Marshal(objSidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field in obj"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var objSid common.SessionID
	if err := objSid.UnmarshalJSON(objSidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field in obj"}
	}

	// Get the counter value
	var objCounter uint64
	switch v := objCntVal.(type) {
	case float64:
		objCounter = uint64(v)
	case int:
		objCounter = uint64(v)
	case int64:
		objCounter = uint64(v)
	case uint64:
		objCounter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number in obj"}
	}

	// Set the target ID
	o.TargetID = common.LogicalTimestamp{SID: objSid, Counter: objCounter}

	if key, ok := op["key"].(string); ok {
		o.Key = key
	} else if startJSON, ok := op["start"]; ok {
		// Get the start in the new format (object with sid and cnt)
		startMap, ok := startJSON.(map[string]interface{})
		if !ok {
			return common.ErrInvalidOperation{Message: "start must be an object with sid and cnt fields"}
		}

		// Get the SID and Counter from the map
		startSidVal, startSidOk := startMap["sid"]
		startCntVal, startCntOk := startMap["cnt"]

		if !startSidOk || !startCntOk {
			return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'start'"}
		}

		// Marshal the SID value to JSON
		startSidJSON, err := json.Marshal(startSidVal)
		if err != nil {
			return common.ErrInvalidOperation{Message: "invalid 'sid' field in start"}
		}

		// Unmarshal the SID using SessionID's UnmarshalJSON
		var startSid common.SessionID
		if err := startSid.UnmarshalJSON(startSidJSON); err != nil {
			return common.ErrInvalidOperation{Message: "invalid 'sid' field in start"}
		}

		// Get the counter value
		var startCounter uint64
		switch v := startCntVal.(type) {
		case float64:
			startCounter = uint64(v)
		case int:
			startCounter = uint64(v)
		case int64:
			startCounter = uint64(v)
		case uint64:
			startCounter = v
		default:
			return common.ErrInvalidOperation{Message: "cnt must be a number in start"}
		}

		// Set the start ID
		o.StartID = common.LogicalTimestamp{SID: startSid, Counter: startCounter}

		// Parse the end field
		endJSON, ok := op["end"]
		if !ok {
			return common.ErrInvalidOperation{Message: "missing 'end' field"}
		}

		// Get the end in the new format (object with sid and cnt)
		endMap, ok := endJSON.(map[string]interface{})
		if !ok {
			return common.ErrInvalidOperation{Message: "end must be an object with sid and cnt fields"}
		}

		// Get the SID and Counter from the map
		endSidVal, endSidOk := endMap["sid"]
		endCntVal, endCntOk := endMap["cnt"]

		if !endSidOk || !endCntOk {
			return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'end'"}
		}

		// Marshal the SID value to JSON
		endSidJSON, err := json.Marshal(endSidVal)
		if err != nil {
			return common.ErrInvalidOperation{Message: "invalid 'sid' field in end"}
		}

		// Unmarshal the SID using SessionID's UnmarshalJSON
		var endSid common.SessionID
		if err := endSid.UnmarshalJSON(endSidJSON); err != nil {
			return common.ErrInvalidOperation{Message: "invalid 'sid' field in end"}
		}

		// Get the counter value
		var endCounter uint64
		switch v := endCntVal.(type) {
		case float64:
			endCounter = uint64(v)
		case int:
			endCounter = uint64(v)
		case int64:
			endCounter = uint64(v)
		case uint64:
			endCounter = v
		default:
			return common.ErrInvalidOperation{Message: "cnt must be a number in end"}
		}

		// Set the end ID
		o.EndID = common.LogicalTimestamp{SID: endSid, Counter: endCounter}
	} else {
		return common.ErrInvalidOperation{Message: "missing 'key' or 'start'/'end' fields"}
	}

	return nil
}

// NopOperation represents a no-op operation.
type NopOperation struct {
	ID        common.LogicalTimestamp
	SpanValue uint64
}

// Type returns the type of the operation.
func (o *NopOperation) Type() common.OperationType {
	return common.OperationTypeNop
}

// GetID returns the ID of the operation.
func (o *NopOperation) GetID() common.LogicalTimestamp {
	return o.ID
}

// Apply applies the operation to the document.
func (o *NopOperation) Apply(doc *crdt.Document) error {
	// No-op operation does nothing
	return nil
}

// Span returns the number of logical clock cycles the operation takes.
func (o *NopOperation) Span() uint64 {
	return o.SpanValue
}

// MarshalJSON returns a JSON representation of the operation.
func (o *NopOperation) MarshalJSON() ([]byte, error) {
	type jsonOp struct {
		Op  string                  `json:"op"`
		ID  common.LogicalTimestamp `json:"id"`
		Len uint64                  `json:"len,omitempty"`
	}

	op := jsonOp{
		Op:  "nop",
		ID:  o.ID,
		Len: o.SpanValue,
	}

	return json.Marshal(op)
}

// UnmarshalJSON parses a JSON representation of the operation.
func (o *NopOperation) UnmarshalJSON(data []byte) error {
	var op map[string]interface{}
	if err := json.Unmarshal(data, &op); err != nil {
		return err
	}

	opStr, ok := op["op"].(string)
	if !ok || opStr != "nop" {
		return common.ErrInvalidOperation{Message: "not a 'nop' operation"}
	}

	// Parse the ID field
	idJSON, ok := op["id"]
	if !ok {
		return common.ErrInvalidOperation{Message: "missing 'id' field"}
	}

	// Get the ID in the new format (object with sid and cnt)
	idMap, ok := idJSON.(map[string]interface{})
	if !ok {
		return common.ErrInvalidOperation{Message: "id must be an object with sid and cnt fields"}
	}

	// Get the SID and Counter from the map
	sidVal, sidOk := idMap["sid"]
	cntVal, cntOk := idMap["cnt"]

	if !sidOk || !cntOk {
		return common.ErrInvalidOperation{Message: "missing 'sid' or 'cnt' field in 'id'"}
	}

	// Marshal the SID value to JSON
	sidJSON, err := json.Marshal(sidVal)
	if err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Unmarshal the SID using SessionID's UnmarshalJSON
	var sid common.SessionID
	if err := sid.UnmarshalJSON(sidJSON); err != nil {
		return common.ErrInvalidOperation{Message: "invalid 'sid' field"}
	}

	// Get the counter value
	var counter uint64
	switch v := cntVal.(type) {
	case float64:
		counter = uint64(v)
	case int:
		counter = uint64(v)
	case int64:
		counter = uint64(v)
	case uint64:
		counter = v
	default:
		return common.ErrInvalidOperation{Message: "cnt must be a number"}
	}

	// Set the operation ID
	o.ID = common.LogicalTimestamp{SID: sid, Counter: counter}

	if len, ok := op["len"].(float64); ok {
		o.SpanValue = uint64(len)
	} else {
		o.SpanValue = 1
	}

	return nil
}
