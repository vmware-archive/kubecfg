package ntlm

import (
	"encoding/hex"

	"golang.org/x/crypto/md4"
)

/*
FromASCIIString calculates the NTLM hash of an ASCII string (in)
*/
func FromASCIIString(in string) []byte {
	/* Prepare a byte array to return */
	var u16 []byte

	/* Add all bytes, as well as the 0x00 of UTF-16 */
	for _, b := range []byte(in) {
		u16 = append(u16, b)
		u16 = append(u16, 0x00)
	}

	/* Hash the byte array with MD4 */
	mdfour := md4.New()
	mdfour.Write(u16)

	/* Return the output */
	return mdfour.Sum(nil)
}

/*
FromASCIIStringToHex calculates the NTLM hash of an ASCII string (in)
and returns it as a hexademical hash in a string, e.g. 00feabcd
*/
func FromASCIIStringToHex(in string) string {
	b := FromASCIIString(in)
	return hex.EncodeToString(b)
}
