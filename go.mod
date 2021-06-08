module github.com/globulario/Globular

go 1.16

replace github.com/globulario/services/golang => ../services/golang

replace github.com/davecourtois/Utility => ../../../github.com/davecourtois/Utility

require (
	github.com/davecourtois/Utility v0.0.0-20210515191918-3118f6f72191
	github.com/fatih/color v1.7.0
	github.com/globulario/services/golang v0.0.0-20210506013013-d8ee75e6a528
	github.com/go-acme/lego v2.7.2+incompatible
	github.com/golang/snappy v0.0.3 // indirect
	github.com/gookit/color v1.4.2
	github.com/kardianos/service v1.2.0
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf // indirect
	golang.org/x/image v0.0.0-20210504121937-7319ad40d33e // indirect
	golang.org/x/sys v0.0.0-20210507161434-a76c4d0a0096 // indirect
	gopkg.in/square/go-jose.v2 v2.5.1 // indirect
)
