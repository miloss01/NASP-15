package MerkleTree

import (
	"io/ioutil"
	"os"
)

type MerkleTree struct {
	root *MerkleNode
	Size int
}

func (mt *MerkleTree) Form(data [][]byte) {
	leafNodes := mt.getLeafNodes(data)
	mt.formOneLevel(leafNodes)
}

func (mt *MerkleTree) BreadthSearch(nodeFunc func(node *MerkleNode)) {
	waitList := MerkleNodeQueue{}
	node := mt.root
	waitList.Enqueue(node)
	for !waitList.IsEmpty() {
		node = waitList.Dequeue()
		if node != nil {
			nodeFunc(node)
			waitList.Enqueue(node.left)
			waitList.Enqueue(node.right)
		}
	}
}

func (mt *MerkleTree) DepthSearch(nodeFunc func(node *MerkleNode)) {
	mt.preOrderDepthSearch(mt.root, nodeFunc)
}

func (mt *MerkleTree) getLeafNodes(data [][]byte) []*MerkleNode {
	leafNodes := make([]*MerkleNode, len(data))
	for i, d := range data {
		node := &MerkleNode{nil, nil, Hash(d)}
		leafNodes[i] = node
	}
	return leafNodes
}

func (mt *MerkleTree) formOneLevel(nodes []*MerkleNode) {
	if len(nodes) == 1 {
		mt.root = &MerkleNode{nil, nil, nodes[0].HashValue}
		return
	}

	mt.Size += len(nodes)
	iterLength := len(nodes)
	parentLength := iterLength / 2
	addEmptyNode := false
	if iterLength%2 == 1 {
		iterLength--
		addEmptyNode = true
		parentLength++
	}

	parentNodes := make([]*MerkleNode, parentLength)
	for i := 0; i < iterLength; i += 2 {
		child1 := nodes[i]
		child2 := nodes[i+1]
		parentHashValue := addHashValues(child1.HashValue, child2.HashValue)
		parent := &MerkleNode{child1, child2, parentHashValue}
		parentNodes[i/2] = parent
	}

	if addEmptyNode {
		emptyNode := &MerkleNode{nil, nil, Hash(nil)}
		parentHashValue := addHashValues(nodes[parentLength-2].HashValue, emptyNode.HashValue)
		parent := &MerkleNode{nodes[iterLength-1], emptyNode, parentHashValue}
		parentNodes[parentLength-1] = parent
		mt.Size++
	}

	if len(parentNodes) > 1 {
		mt.formOneLevel(parentNodes)
	} else {
		mt.root = parentNodes[0]
		mt.Size++
	}
}

func (mt *MerkleTree) preOrderDepthSearch(node *MerkleNode, nodeFunc func(node *MerkleNode)) {
	if node != nil {
		nodeFunc(node)
		mt.preOrderDepthSearch(node.left, nodeFunc)
		mt.preOrderDepthSearch(node.right, nodeFunc)
	}
}

func (mt *MerkleTree) Serialize(filePath string) {
	serIndex := 0
	hashValuesForSerialization := make([]byte, 20*mt.Size)
	mt.BreadthSearch(func(node *MerkleNode) {
		for i := 0; i < 20; i++ {
			hashValuesForSerialization[20*serIndex+i] = node.HashValue[i]
		}
		serIndex++
	})
	file, err := os.OpenFile(filePath, os.O_WRONLY, 0222)
	if err != nil {
		file, err = os.Create(filePath)
		if err != nil {
			panic(err)
		}
	}
	defer file.Close()

	file.Write(hashValuesForSerialization)
	file.Sync()
}

func (mt *MerkleTree) Deserialize(filePath string) {
	file, err := os.OpenFile(filePath, os.O_RDONLY, 0444)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	hashValues, err := ioutil.ReadAll(file)
	if err != nil {
		panic(err)
	}
	nodes := make([]*MerkleNode, 0)
	for i := 0; i < len(hashValues); i += 20 {
		var hashValue [20]byte
		for j := 0; j < 20; j++ {
			hashValue[j] = hashValues[i : i+20][j]
		}
		nodes = append(nodes, &MerkleNode{nil, nil, hashValue})
	}
	childIndex := 1
	for i := 0; i < len(nodes); i++ {
		if i+childIndex >= len(nodes) {
			break
		}
		nodes[i].SetLeftNode(nodes[i+childIndex])
		nodes[i].SetRightNode(nodes[i+1+childIndex])
		childIndex++
	}
	mt.root = nodes[0]
}