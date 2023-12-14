package afs

// OsFS should be implemented by Filesytem implementations that write to the OS filesystem.
type OsFS interface {
	OsFs()
}
