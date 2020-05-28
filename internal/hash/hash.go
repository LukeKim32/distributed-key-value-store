package hash

import "github.com/howeyc/crc16"

/* CRC key = 16384 = 2^14
 * In polynomial : x^14
 * In Hex : 0x4000
 */
const (
	// HashSlotsNumber is identical with the number of Redis cluster hash slots
	HashSlotsNumber = 16384
)

var checkSumTable *crc16.Table

func init() {
	if checkSumTable == nil {
		// CRC16-CCITT 를 이용하여 Table을 만든다.
		checkSumTable = crc16.MakeTable(crc16.CCITT)
	}
}

// GetHashSlotIndex gets the index of Hash Slots
// By using CRC16 with @data and Modulo 16384 (Like Redis Cluster)
func GetHashSlotIndex(data string) uint16 {

	// Redis는 CRC16 의 Modulo 16384를 사용한다.
	hashSlotIndex := crc16.Checksum([]byte(data), checkSumTable) % HashSlotsNumber

	return hashSlotIndex
}
