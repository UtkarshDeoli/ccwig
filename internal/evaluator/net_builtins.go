package evaluator

import (
	"github.com/UtkarshDeoli/ccwig/internal/object"
	"github.com/UtkarshDeoli/ccwig/internal/sockets"
	"github.com/UtkarshDeoli/ccwig/internal/token"
)

// The networking builtins. They are shaped like C's socket calls rather than
// like the rest of this language: a connection is an integer descriptor, a
// failed call returns -1 (or null), and `net_error` holds the reason.
//
// The split is deliberate. A wrong *type* — passing a string where a port
// belongs — is a mistake in the program and is reported as an error that stops
// it, as everywhere else. A failed *operation* — a refused connection, a
// closed port, a timeout — is something a working program expects and handles,
// so it comes back as a value. That is why `if fd < 0 { ... }` reads the same
// here as it does in C.
const (
	builtinNetListen     = "net_listen"
	builtinNetAccept     = "net_accept"
	builtinNetConnect    = "net_connect"
	builtinNetSend       = "net_send"
	builtinNetRecv       = "net_recv"
	builtinNetRecvLine   = "net_recvline"
	builtinNetClose      = "net_close"
	builtinNetSetTimeout = "net_settimeout"
	builtinNetAddr       = "net_addr"
	builtinNetError      = "net_error"
	builtinNetTimedOut   = "net_timedout"
)

func (e *Evaluator) netBuiltins() map[string]*object.Builtin {
	return map[string]*object.Builtin{
		builtinNetListen: {
			Name: builtinNetListen,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetListen, args, 1, token.Position{}); err != nil {
					return err
				}
				port, errObj := integerArg(builtinNetListen, args[0], "port")
				if errObj != nil {
					return errObj
				}
				return &object.Integer{Value: int64(e.net().Listen(int(port)))}
			},
		},

		builtinNetAccept: {
			Name: builtinNetAccept,
			// Blocks until a client arrives. A socket with a timeout set
			// gives up instead, which is what a server that must not hang
			// relies on.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetAccept, args, 1, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetAccept, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				return &object.Integer{Value: int64(e.net().Accept(int(fd)))}
			},
		},

		builtinNetConnect: {
			Name: builtinNetConnect,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetConnect, args, 2, token.Position{}); err != nil {
					return err
				}
				host, ok := args[0].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "first argument to `net_connect` must be a string, got %s", args[0].Type())
				}
				port, errObj := integerArg(builtinNetConnect, args[1], "port")
				if errObj != nil {
					return errObj
				}
				return &object.Integer{Value: int64(e.net().Connect(host.Value, int(port)))}
			},
		},

		builtinNetSend: {
			Name: builtinNetSend,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetSend, args, 2, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetSend, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				data, ok := args[1].(*object.String)
				if !ok {
					return object.NewError(token.Position{}, "second argument to `net_send` must be a string, got %s", args[1].Type())
				}
				return &object.Integer{Value: int64(e.net().Send(int(fd), data.Value))}
			},
		},

		builtinNetRecv: {
			Name: builtinNetRecv,
			// Up to `max` bytes as a string, or null at the end of the stream.
			// A failure is also null, because there are only so many ways to
			// say nothing; `net_error` distinguishes the two for a caller that
			// cares, and most callers only care that the data stopped.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetRecv, args, 2, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetRecv, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				max, errObj := integerArg(builtinNetRecv, args[1], "size")
				if errObj != nil {
					return errObj
				}

				data, eof := e.net().Recv(int(fd), int(max))
				if eof || e.net().LastError() != "" {
					return object.NullValue()
				}
				return &object.String{Value: data}
			},
		},

		builtinNetRecvLine: {
			Name: builtinNetRecvLine,
			// A line without its newline, or null at the end of the stream.
			// Line protocols such as HTTP and SMTP are far easier to speak
			// with this than with a byte count and a buffer of one's own.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetRecvLine, args, 1, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetRecvLine, args[0], "socket")
				if errObj != nil {
					return errObj
				}

				line, eof := e.net().RecvLine(int(fd))
				if eof || e.net().LastError() != "" {
					return object.NullValue()
				}
				return &object.String{Value: line}
			},
		},

		builtinNetClose: {
			Name: builtinNetClose,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetClose, args, 1, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetClose, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				e.net().Close(int(fd))
				return object.NullValue()
			},
		},

		builtinNetSetTimeout: {
			Name: builtinNetSetTimeout,
			// A deadline for the next read, write or accept on this socket.
			// Zero removes it. This is what keeps a benchmark — or a test, or
			// a server talking to a client that went away — from waiting
			// forever.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetSetTimeout, args, 2, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetSetTimeout, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				ms, errObj := integerArg(builtinNetSetTimeout, args[1], "milliseconds")
				if errObj != nil {
					return errObj
				}
				e.net().SetTimeout(int(fd), int(ms))
				return object.NullValue()
			},
		},

		builtinNetAddr: {
			Name: builtinNetAddr,
			// A listening socket reports the address it is bound to, which is
			// how a server that asked for port 0 finds out which port it got.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetAddr, args, 1, token.Position{}); err != nil {
					return err
				}
				fd, errObj := integerArg(builtinNetAddr, args[0], "socket")
				if errObj != nil {
					return errObj
				}
				return &object.String{Value: e.net().Addr(int(fd))}
			},
		},

		builtinNetError: {
			Name: builtinNetError,
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetError, args, 0, token.Position{}); err != nil {
					return err
				}
				return &object.String{Value: e.net().LastError()}
			},
		},

		builtinNetTimedOut: {
			Name: builtinNetTimedOut,
			// Whether the last failure was a deadline passing, rather than the
			// socket breaking.
			//
			// This is the question a server polling many clients has to ask
			// after every empty read, and it is a different question from
			// `net_error`: "nothing has arrived yet, ask again" is not an
			// error at all. Reading the answer out of the error message would
			// mean matching on text the operating system chose, which is a
			// thing a language should not make its programs do.
			Fn: func(args []object.Object) object.Object {
				if err := expectArgs(builtinNetTimedOut, args, 0, token.Position{}); err != nil {
					return err
				}
				return object.BooleanOf(e.net().TimedOut())
			},
		},
	}
}

// net returns the socket registry, creating it on first use.
//
// One registry belongs to one evaluator, so a program's open sockets survive
// from one statement to the next, and the descriptors a program sees are the
// same ones `net_close` later accepts.
func (e *Evaluator) net() *sockets.Registry {
	if e.socks == nil {
		e.socks = sockets.NewRegistry()
	}
	return e.socks
}

// integerArg reads a builtin's integer argument, naming what it was for in the
// message.
func integerArg(name string, arg object.Object, what string) (int64, *object.Error) {
	n, ok := arg.(*object.Integer)
	if !ok {
		return 0, object.NewError(token.Position{}, "argument to `%s` for the %s must be an integer, got %s", name, what, arg.Type())
	}
	return n.Value, nil
}
