package LSMCompaction

import (
	"Projekat/Structures/BloomFilter"
	"Projekat/Structures/MerkleTree"
	"encoding/binary"
	"io"
	"io/ioutil"
	"os"
	"strconv"
	"strings"
)

const (
	maxLSMLevel = 10
	maxSSTables = 4
)

func LSMCompaction(lsmLevel int) {
	ssTableNames := getSSTableNamesByLevel(lsmLevel)
	if len(ssTableNames) < maxSSTables || lsmLevel >= maxLSMLevel {
		return
	}

	deleteMerkleTreesByLevel(ssTableNames, lsmLevel)
	deleteBloomFilterByLevel(ssTableNames, lsmLevel)

	mergeSSTables(ssTableNames, lsmLevel)

	mergedSSTableName := getSSTableNamesByLevel(lsmLevel)[0]
	availableSerialNumFromNextLSMLevel := getAvailableSerialNumFromNextLSMLevel(lsmLevel)
	newSSTableName := "Data_lvl" + strconv.Itoa(lsmLevel+1) + "_" + availableSerialNumFromNextLSMLevel + ".db"

	_ = os.Rename("./Data/"+mergedSSTableName, "./Data/"+newSSTableName)

	createNewMerkleTree(newSSTableName)
	createNewBloomFilter(newSSTableName)

	LSMCompaction(lsmLevel + 1)
}

// mergeSSTables
// Merges SSTables that are passed on as parameters (names of SSTable files)
// Merges all SSTables from same level
// Similar to merge sort algorithm
func mergeSSTables(ssTables []string, lsmLevel int) {
	iterLength := len(ssTables)
	if iterLength%2 == 1 {
		iterLength--
	}
	for i := 0; i < iterLength; i += 2 {
		ssTableFile1, err1 := os.OpenFile("./Data/"+ssTables[i], os.O_RDONLY, 0444)
		if err1 != nil {
			panic(err1)
		}
		ssTableFile2, err2 := os.OpenFile("./Data/"+ssTables[i+1], os.O_RDONLY, 0444)
		if err2 != nil {
			panic(err2)
		}

		newFileSerialNum := getDataFileNameSerialNum(ssTables[i]) + "-" + getDataFileNameSerialNum(ssTables[i+1])
		mergeTwoSSTables(ssTableFile1, ssTableFile2, lsmLevel, newFileSerialNum, iterLength == 2)

		_ = ssTableFile1.Close()
		_ = ssTableFile2.Close()
		err := os.Remove(ssTableFile1.Name())
		if err != nil {
			panic(err)
		}
		err = os.Remove(ssTableFile2.Name())
		if err != nil {
			panic(err)
		}
	}

	if iterLength > 2 {
		mergeSSTables(getSSTableNamesByLevel(lsmLevel), lsmLevel)
	}
}

// mergeTwoSSTables
// doDelete is an indicator to do physical removal of items
// to ensure the item is removed, removal needs to be applicable only when merging the last two SSTables
// if an item gets removed before that, there is a chance there will be an older version of the item
// that has tombstone = 0 which managed to skip on merging with the file where that item has tombstone != 0
// in which case the item will still be in the final merged SSTable
func mergeTwoSSTables(ssTableFile1, ssTableFile2 *os.File, lsmLevel int, newFileSerialNum string, doDelete bool) {
	newSSTableFile, err := os.Create("./Data/Data_lvl" + strconv.Itoa(lsmLevel) + "_" + newFileSerialNum + ".db")
	if err != nil {
		panic(err)
	}

	ssTableElement1, err1 := getNextSSTableElement(ssTableFile1)
	ssTableElement2, err2 := getNextSSTableElement(ssTableFile2)
	for {
		if err1 == io.EOF && err2 == io.EOF {
			break
		}
		if err1 == io.EOF {
			if ssTableElement2.Tombstone[0] == 0 || !doDelete {
				_, _ = newSSTableFile.Write(ssTableElement2.GetAsByteArray())
			}
			ssTableElement2, err2 = getNextSSTableElement(ssTableFile2)
			continue
		}
		if err2 == io.EOF {
			if ssTableElement1.Tombstone[0] == 0 || !doDelete {
				_, _ = newSSTableFile.Write(ssTableElement1.GetAsByteArray())
			}
			ssTableElement1, err1 = getNextSSTableElement(ssTableFile1)
			continue
		}

		if ssTableElement1.GetKey() < ssTableElement2.GetKey() {
			if ssTableElement1.Tombstone[0] == 0 || !doDelete {
				_, _ = newSSTableFile.Write(ssTableElement1.GetAsByteArray())
			}
			ssTableElement1, err1 = getNextSSTableElement(ssTableFile1)
		} else if ssTableElement1.GetKey() > ssTableElement2.GetKey() {
			if ssTableElement2.Tombstone[0] == 0 || !doDelete {
				_, _ = newSSTableFile.Write(ssTableElement2.GetAsByteArray())
			}
			ssTableElement2, err2 = getNextSSTableElement(ssTableFile2)
		} else {
			if ssTableElement1.CheckNewer(ssTableElement2) {
				if ssTableElement1.Tombstone[0] == 0 || !doDelete {
					_, _ = newSSTableFile.Write(ssTableElement1.GetAsByteArray())
				}
			} else {
				if ssTableElement2.Tombstone[0] == 0 || !doDelete {
					_, _ = newSSTableFile.Write(ssTableElement2.GetAsByteArray())
				}
			}
			ssTableElement1, err1 = getNextSSTableElement(ssTableFile1)
			ssTableElement2, err2 = getNextSSTableElement(ssTableFile2)
		}
	}

	_ = newSSTableFile.Close()
}

func getNextSSTableElement(ssTableFile *os.File) (SSTableElement, error) {
	ssTableElBytes := make([]byte, 37)
	_, err := ssTableFile.Read(ssTableElBytes)
	if err != nil {
		if err == io.EOF {
			return SSTableElement{}, err
		} else {
			panic(err)
		}
	}
	keySize := binary.BigEndian.Uint64(ssTableElBytes[21:29])
	valueSize := binary.BigEndian.Uint64(ssTableElBytes[29:37])

	offset := 37 + keySize + valueSize
	ssTableElBytes = make([]byte, offset)
	_, _ = ssTableFile.Seek(-37, 1)
	_, err = ssTableFile.Read(ssTableElBytes)
	if err != nil {
		panic(err)
	}

	return createSSTableElement(ssTableElBytes), nil
}

func getSSTableNamesByLevel(lsmLevel int) []string {
	allDataFiles, err := ioutil.ReadDir("./Data/")
	if err != nil {
		panic(err)
	}
	ssTables := make([]string, 0)
	for _, file := range allDataFiles {
		if strings.Contains(file.Name(), "Data_lvl"+strconv.Itoa(lsmLevel)) {
			ssTables = append(ssTables, file.Name())
		}
	}

	return ssTables
}

func createSSTableElement(data []byte) SSTableElement {
	ssTableElement := SSTableElement{}

	var crc [4]byte
	for i, b := range data[:4] {
		crc[i] = b
	}
	ssTableElement.CRC = crc

	var timestamp [16]byte
	for i, b := range data[4:20] {
		timestamp[i] = b
	}
	ssTableElement.Timestamp = timestamp

	var tombstone [1]byte
	for i, b := range data[20:21] {
		tombstone[i] = b
	}
	ssTableElement.Tombstone = tombstone

	var keySize [8]byte
	for i, b := range data[21:29] {
		keySize[i] = b
	}
	ssTableElement.KeySize = keySize

	var valueSize [8]byte
	for i, b := range data[29:37] {
		valueSize[i] = b
	}
	ssTableElement.ValueSize = valueSize

	ssTableElement.Key = data[37 : 37+ssTableElement.GetKeySize()]
	ssTableElement.Value = data[37+ssTableElement.GetKeySize():]
	return ssTableElement
}

// getDataFileNameSerialNum example: if name is "Data_lvl1_2.db" returns 2
func getDataFileNameSerialNum(ssTableName string) string {
	splitByUnderscore := strings.Split(ssTableName, "_")
	serialNum := splitByUnderscore[2]
	serialNum = strings.ReplaceAll(serialNum, ".db", "")
	return serialNum
}

func getAvailableSerialNumFromNextLSMLevel(lsmLevel int) string {
	ssTablesFromNextLevel := getSSTableNamesByLevel(lsmLevel + 1)
	return strconv.Itoa(len(ssTablesFromNextLevel) + 1)
}

func deleteMerkleTreesByLevel(ssTables []string, lsmLevel int) {
	for _, ssTable := range ssTables {
		err := os.Remove("./Data/MerkleTree_lvl" + strconv.Itoa(lsmLevel) + "_" + getDataFileNameSerialNum(ssTable) + ".db")
		if err != nil {
			panic(err)
		}
	}
}

func createNewMerkleTree(ssTableName string) {
	ssTableFile, err := os.OpenFile("./Data/"+ssTableName, os.O_RDONLY, 0444)
	if err != nil {
		panic(err)
	}
	ssTableData := make([][]byte, 0)
	for {
		ssTableElement, err := getNextSSTableElement(ssTableFile)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		ssTableData = append(ssTableData, ssTableElement.Key)

	}
	_ = ssTableFile.Close()

	merkleTree := MerkleTree.MerkleTree{}
	merkleTree.Form(ssTableData)
	ssTableNameSplitUnderscore := strings.Split(ssTableName, "_")
	merkleTree.Serialize("./Data/MerkleTree_" + strings.Join(ssTableNameSplitUnderscore[1:], "_"))
}

func deleteBloomFilterByLevel(ssTables []string, lsmLevel int) {
	for _, ssTable := range ssTables {
		err := os.Remove("./Data/BloomFilter_lvl" + strconv.Itoa(lsmLevel) + "_" + getDataFileNameSerialNum(ssTable) + ".db")
		if err != nil {
			panic(err)
		}
	}
}

func createNewBloomFilter(ssTableName string) {
	ssTableFile, err := os.OpenFile("./Data/"+ssTableName, os.O_RDONLY, 0444)
	bloomFilterKeys := make([]string, 0)
	if err != nil {
		panic(err)
	}
	for {
		ssTableElement, err := getNextSSTableElement(ssTableFile)
		if err != nil {
			if err == io.EOF {
				break
			} else {
				panic(err)
			}
		}
		bloomFilterKeys = append(bloomFilterKeys, ssTableElement.GetKey())
	}
	_ = ssTableFile.Close()

	newBloomFilter := BloomFilter.MakeBloomFilter(len(bloomFilterKeys), 0.1)
	for _, key := range bloomFilterKeys {
		newBloomFilter.Add(key)
	}

	ssTableNameSplitUnderscore := strings.Split(ssTableName, "_")
	newBloomFilterFile, err := os.Create("./Data/BloomFilter_" + strings.Join(ssTableNameSplitUnderscore[1:], "_"))
	if err != nil {
		panic(err)
	}
	_, _ = newBloomFilterFile.Write(newBloomFilter.Serialize())
	_ = newBloomFilterFile.Close()
}