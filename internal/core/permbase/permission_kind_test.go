package permbase

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissionKind(t *testing.T) {

	t.Run("IsMajor & IsMinor", func(t *testing.T) {
		assert.True(t, PermissionKind(0).IsMajor())
		assert.False(t, PermissionKind(0).IsMinor())

		assert.True(t, PermissionKind(1).IsMajor())
		assert.False(t, PermissionKind(1).IsMinor())

		assert.True(t, PermissionKind(256).IsMajor())
		assert.False(t, PermissionKind(256).IsMinor())

		assert.True(t, PermissionKind(65_535).IsMajor())
		assert.False(t, PermissionKind(65_535).IsMinor())

		assert.False(t, PermissionKind(65_535+1).IsMajor())
		assert.True(t, PermissionKind(65_535+1).IsMinor())

		assert.False(t, PermissionKind(1+(1<<16)).IsMajor())
		assert.True(t, PermissionKind(1+(1<<16)).IsMinor())
	})

	t.Run("Major", func(t *testing.T) {
		//major
		assert.Equal(t, PermissionKind(0), PermissionKind(0).Major())
		assert.Equal(t, PermissionKind(1), PermissionKind(1).Major())
		assert.Equal(t, PermissionKind(256), PermissionKind(256).Major())
		assert.Equal(t, PermissionKind(65_535), PermissionKind(65_535).Major())

		//minor
		assert.Equal(t, PermissionKind(0), PermissionKind(65_535+1).Major())
		assert.Equal(t, PermissionKind(1), PermissionKind(1+(1<<16)).Major())
	})

	t.Run("Includes", func(t *testing.T) {
		//major
		assert.True(t, PermissionKind(0).Includes(PermissionKind(0)))
		assert.True(t, PermissionKind(0).Includes(PermissionKind(0+(1<<16))))

		assert.True(t, PermissionKind(1).Includes(PermissionKind(1)))
		assert.True(t, PermissionKind(1).Includes(PermissionKind(1+(1<<16))))

		assert.True(t, PermissionKind(256).Includes(PermissionKind(256)))
		assert.True(t, PermissionKind(256).Includes(PermissionKind(256+(1<<16))))

		assert.True(t, PermissionKind(65_535).Includes(PermissionKind(65_535)))
		assert.True(t, PermissionKind(65_535).Includes(PermissionKind(65_535+(1<<16))))

		//minor
		assert.True(t, PermissionKind(0+(1<<16)).Includes(PermissionKind(0+(1<<16))))
		assert.False(t, PermissionKind(0+(1<<16)).Includes(PermissionKind(0)))

		assert.True(t, PermissionKind(1+(1<<16)).Includes(PermissionKind(1+(1<<16))))
		assert.False(t, PermissionKind(1+(1<<16)).Includes(PermissionKind(1)))

		assert.True(t, PermissionKind(256+(1<<16)).Includes(PermissionKind(256+(1<<16))))
		assert.False(t, PermissionKind(256+(1<<16)).Includes(PermissionKind(256)))

		assert.True(t, PermissionKind(65_535+(1<<16)).Includes(PermissionKind(65_535+(1<<16))))
		assert.False(t, PermissionKind(65_535+(1<<16)).Includes(PermissionKind(65_535)))
	})

}
