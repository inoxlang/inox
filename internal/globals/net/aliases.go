package internal

import (
	core "github.com/inoxlang/inox/internal/core"
	symbolic "github.com/inoxlang/inox/internal/core/symbolic"
)

//values

type Value = core.Value

type Int = core.Int
type FileMode = core.FileMode
type Float = core.Float
type Date = core.Date
type Bool = core.Bool
type Rune = core.Rune
type Byte = core.Byte
type ByteCount = core.ByteCount
type ByteRate = core.ByteRate
type ByteSlice = core.ByteSlice
type RuneSlice = core.RuneSlice
type CheckedString = core.CheckedString
type Str = core.Str
type Identifier = core.Identifier
type Object = core.Object
type List = core.List
type KeyList = core.KeyList
type Dictionary = core.Dictionary
type Iterable = core.Iterable
type Indexable = core.Indexable
type Iterator = core.Iterator
type WrappedString = core.WrappedString
type IBytes = core.WrappedBytes
type Option = core.Option
type RoutineGroup = core.RoutineGroup
type Routine = core.Routine
type AstNode = core.AstNode
type QuantityRange = core.QuantityRange
type IntRange = core.IntRange
type Path = core.Path
type PathPattern = core.PathPattern
type Pattern = core.Pattern
type PatternFn = core.TypePattern
type ObjectPattern = core.ObjectPattern
type Duration = core.Duration
type URL = core.URL
type URLPattern = core.URLPattern
type Scheme = core.Scheme
type Host = core.Host
type HostPattern = core.HostPattern
type Record = core.Record

type GoValue = core.GoValue
type GoFunction = core.GoFunction
type InoxFunction = core.InoxFunction
type Reader = core.Reader
type Readable = core.Readable
type ResourceName = core.ResourceName
type Mimetype = core.Mimetype
type FileInfo = core.FileInfo
type Effect = core.Effect
type Reversability = core.Reversability

//symbolic

type SymbolicValue = symbolic.SymbolicValue
type SymbolicString = symbolic.String
type SymbolicInt = symbolic.Int

//permissions

type Permission = core.Permission
type PermissionKind = core.PermissionKind
type Limitation = core.Limitation
type LimitationKInd = core.LimitationKind

type CommandPermission = core.CommandPermission
type DNSPermission = core.DNSPermission
type EnvVarPermission = core.EnvVarPermission
type FilesystemPermission = core.FilesystemPermission
type GlobalVarPermission = core.GlobalVarPermission
type HttpPermission = core.HttpPermission
type RawTcpPermission = core.RawTcpPermission
type RoutinePermission = core.RoutinePermission
type WebsocketPermission = core.WebsocketPermission

//state

type GlobalState = core.GlobalState
type Context = core.Context
type ContextConfig = core.ContextConfig

//events

type EventSource = core.EventSource
type EventHandler = core.EventHandler

//other

type CheckInput = core.StaticCheckInput
type PrettyPrintConfig = core.PrettyPrintConfig
