module github.com/oluies/gjallarhorn

go 1.25.0

// The vuvuzela.io vanity import domain is currently unreachable.
// Map the sister modules to their canonical GitHub repositories so
// builds work without depending on the vanity redirector. Mirrors the
// equivalent block in github.com/oluies/neverlur's go.mod.
replace (
	// vuvuzela.io/alpenhorn is no longer used by any Gjallarhorn Go
	// file (the rebrand moved every consumer to
	// github.com/oluies/neverlur). It is, however, a transitive
	// requirement of the inherited vuvuzela.io/vuvuzela module, so
	// the replace stays to point it at the pre-rebrand Vuvuzela fork
	// snapshot — the vanity domain is unreachable, and we cannot
	// alias it as github.com/oluies/neverlur because Go forbids the
	// same module being used for two paths.
	//
	// Workspace mode (per docs/local-development.md) overrides this
	// with a workspace-level `replace vuvuzela.io/alpenhorn => ./neverlur`
	// in go.work to avoid the cross-repo `replace` conflict against
	// Neverlur's own `replace vuvuzela.io/alpenhorn => ./`.
	// See specs/001-conversation-wiring/research.md R2.
	vuvuzela.io/alpenhorn => github.com/vuvuzela/alpenhorn v0.0.0-20190912152808-6b33518f681e
	vuvuzela.io/concurrency => github.com/vuvuzela/concurrency v0.0.0-20190327123758-e608f351e310
	vuvuzela.io/crypto => github.com/vuvuzela/crypto v0.0.0-20220523120157-1709ed3a3b66
	vuvuzela.io/internal => github.com/vuvuzela/internal v0.0.0-20190910144301-7321cf92c8ba
	vuvuzela.io/vuvuzela => github.com/vuvuzela/vuvuzela v0.0.0-20190912153956-55ba49f81ad0
)

require (
	github.com/davidlazar/easyjson v0.0.0-20170924022152-f8e31516abf8
	github.com/davidlazar/go-crypto v0.0.0-20200604182044-b73af7476f6c
	github.com/gen2brain/beeep v0.11.2
	github.com/gogo/protobuf v1.3.2
	github.com/jroimartin/gocui v0.5.0
	github.com/oluies/neverlur v0.0.0-20260524082542-4658aac559bb
	golang.org/x/crypto v0.52.0
	golang.org/x/net v0.55.0
	google.golang.org/grpc v1.81.1
	vuvuzela.io/concurrency v0.0.0-00010101000000-000000000000
	vuvuzela.io/crypto v0.0.0-00010101000000-000000000000
)

require (
	git.sr.ht/~jackmordaunt/go-toast v1.1.2 // indirect
	github.com/AndreasBriese/bbloom v0.0.0-20190825152654-46b345b51c96 // indirect
	github.com/cespare/xxhash v1.1.0 // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/davidlazar/mapstructure v0.0.0-20170906201703-c9d7ddc4ff97 // indirect
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/dgraph-io/badger v1.6.2 // indirect
	github.com/dgraph-io/ristretto v0.0.2 // indirect
	github.com/dustin/go-humanize v1.0.0 // indirect
	github.com/esiqveland/notify v0.13.3 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.1.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/jackmordaunt/icns/v3 v3.0.1 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.22 // indirect
	github.com/mattn/go-runewidth v0.0.9 // indirect
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646 // indirect
	github.com/nsf/termbox-go v1.1.1 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	github.com/sergeymakinen/go-bmp v1.0.0 // indirect
	github.com/sergeymakinen/go-ico v1.0.0-beta.0 // indirect
	github.com/tadvi/systray v0.0.0-20190226123456-11a2b8fa57af // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260226221140-a57be14db171 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	vuvuzela.io/alpenhorn v0.0.0-00010101000000-000000000000 // indirect
	vuvuzela.io/vuvuzela v0.0.0-00010101000000-000000000000 // indirect
)
