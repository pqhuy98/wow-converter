package casc

// Content flags mirrored from wow.export/src/js/casc/content-flags.js
const (
	ContentLoadOnWindows      = 0x8
	ContentLoadOnMacOS        = 0x10
	ContentLowViolence        = 0x80
	ContentDoNotLoad          = 0x100
	ContentUpdatePlugin       = 0x800
	ContentEncrypted          = 0x08000000
	ContentNoNameHash         = 0x10000000
	ContentUncommonResolution = 0x20000000
	ContentBundle             = 0x40000000
	ContentNoCompression      = 0x80000000
)
