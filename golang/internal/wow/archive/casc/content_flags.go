package casc

// ContentFlags are CASC content flags.
type ContentFlags uint32

const (
	ContentLoadOnWindows       ContentFlags = 0x8
	ContentLoadOnMacOS         ContentFlags = 0x10
	ContentLowViolence         ContentFlags = 0x80
	ContentDoNotLoad           ContentFlags = 0x100
	ContentUpdatePlugin        ContentFlags = 0x800
	ContentEncrypted           ContentFlags = 0x8000000
	ContentNoNameHash          ContentFlags = 0x10000000
	ContentUncommonResolution  ContentFlags = 0x20000000
	ContentBundle              ContentFlags = 0x40000000
	ContentNoCompression       ContentFlags = 0x80000000
)
