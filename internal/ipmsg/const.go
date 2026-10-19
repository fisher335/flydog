package ipmsg

const (
	// Version is the IPMSG protocol version we speak.
	Version = 1

	// Port is the default UDP/TCP port used by IPMSG.
	Port = 2425

	// FileListSeparator separates attachment entries inside the extra section.
	FileListSeparator = 0x07

	// HostListSeparator is the ':' used to separate packet header fields.
	HostListSeparator = ':'

	// MaxUDPPacket is a safe upper bound for a UDP datagram we read.
	MaxUDPPacket = 65507

	// MaxHeader is the largest file-info header we accept.
	MaxHeader = 8192
)

// Command modes (low 8 bits of the command word).
const (
	CmdNoOperation     = 0x00000000
	CmdBrEntry         = 0x00000001
	CmdBrExit          = 0x00000002
	CmdAnsEntry        = 0x00000003
	CmdBrAbsence       = 0x00000004
	CmdBrIsGetList     = 0x00000010
	CmdOkGetList       = 0x00000011
	CmdGetList         = 0x00000012
	CmdAnsList         = 0x00000013
	CmdBrIsGetList2    = 0x00000018
	CmdSendMsg         = 0x00000020
	CmdRecvMsg         = 0x00000021
	CmdReadMsg         = 0x00000030
	CmdDelMsg          = 0x00000031
	CmdAnsReadMsg      = 0x00000032
	CmdGetInfo         = 0x00000040
	CmdSendInfo        = 0x00000041
	CmdGetAbsenceInfo  = 0x00000050
	CmdSendAbsenceInfo = 0x00000051
	CmdGetFileData     = 0x00000060
	CmdReleaseFiles    = 0x00000061
	CmdGetDirFiles     = 0x00000062
	CmdGetPubKey       = 0x00000072
	CmdAnsPubKey       = 0x00000073
)

// 飞秋 extensions.
const (
	CmdOpenYou   = 0x00000077 // "I opened a chat with you"
	CmdInputing  = 0x00000079 // peer is typing
	CmdInputEnd  = 0x0000007a // peer stopped typing
	CmdSendImage = 0x000000c0 // inline image
	CmdKnock     = 0x000000d1 // window knock / shake
)

// Option flags (high 24 bits of the command word).
const (
	OptAbsence     = 0x00000100
	OptServer      = 0x00000200
	OptDialup      = 0x00010000
	OptFileAttach  = 0x00200000
	OptEncrypt     = 0x00400000
	OptCapUTF8     = 0x00800000 // peer can speak UTF-8
	OptUTF8        = 0x00800000 // this packet uses UTF-8
	OptSendCheck   = 0x00000100
	OptSecret      = 0x00000200
	OptBroadcast   = 0x00000400
	OptMulticast   = 0x00000800
	OptNoPopup     = 0x00001000
	OptAutoRet     = 0x00002000
	OptRetry       = 0x00004000
	OptPassword    = 0x00008000
	OptNoLog       = 0x00020000
	OptNewMuti     = 0x00040000
	OptNoAddList   = 0x00080000
	OptReadCheck   = 0x00100000
	OptSecretEx    = OptReadCheck | OptSecret
	OptNoReplyOpts = OptBroadcast | OptAutoRet
)

// Attachment file types (low 8 bits of fileattr).
const (
	FileRegular   = 0x00000001
	FileDir       = 0x00000002
	FileRetParent = 0x00000003
	FileSymlink   = 0x00000004
	FileCDev      = 0x00000005
	FileBDev      = 0x00000006
	FileFIFO      = 0x00000007
	FileResFork   = 0x00000010
)

// Attachment file attribute options (high bits of fileattr).
const (
	FileROnlyOpt    = 0x00000100
	FileHiddenOpt   = 0x00001000
	FileExHiddenOpt = 0x00002000
	FileArchiveOpt  = 0x00004000
	FileSystemOpt   = 0x00008000
)

// Extended attribute types for the file-info string.
const (
	ExtAttrUID          = 0x00000001
	ExtAttrUserName     = 0x00000002
	ExtAttrGID          = 0x00000003
	ExtAttrGroupName    = 0x00000004
	ExtAttrPerm         = 0x00000010
	ExtAttrMajorNo      = 0x00000011
	ExtAttrMinorNo      = 0x00000012
	ExtAttrCTime        = 0x00000013
	ExtAttrMTime        = 0x00000014
	ExtAttrATime        = 0x00000015
	ExtAttrCreateTime   = 0x00000016
	ExtAttrCreator      = 0x00000020
	ExtAttrFileType     = 0x00000021
	ExtAttrFinderInfo   = 0x00000022
	ExtAttrACL          = 0x00000030
	ExtAttrAliasFName   = 0x00000040
	ExtAttrUnicodeFName = 0x00000041
)

// GetMode returns the command mode (low 8 bits).
func GetMode(cmd int) int { return cmd & 0xff }

// GetOpt returns the option flags (high 24 bits).
func GetOpt(cmd int) int { return cmd & 0xffffff00 }

// IsSet reports whether every bit in opt is set in cmd.
func IsSet(cmd, opt int) bool { return cmd&opt == opt }

// IsMode reports whether the command mode equals mode.
func IsMode(cmd, mode int) bool { return GetMode(cmd) == mode }
