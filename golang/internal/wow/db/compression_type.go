package db

// CompressionType describes DB2 field compression modes.
type CompressionType int

const (
	CompressionNone CompressionType = iota
	CompressionBitpacked
	CompressionCommonData
	CompressionBitpackedIndexed
	CompressionBitpackedIndexedArray
	CompressionBitpackedSigned
)
