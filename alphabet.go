package base64

// Error es un error de base64 sin dependencias. El paquete presume de cero
// imports (ver README: importar webtyp/fmt cuesta 74 KB en TinyGo) y por
// eso no usa fmt.Errorf ni errors.New.
type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrAlphabetLength    = Error("base64: el alfabeto debe tener exactamente 64 caracteres")
	ErrAlphabetDuplicate = Error("base64: el alfabeto tiene caracteres repetidos")
	ErrAlphabetNonASCII  = Error("base64: el alfabeto sólo admite ASCII")
	ErrInvalidCharacter  = Error("base64: carácter no válido en la entrada")
	ErrInvalidLength     = Error("base64: longitud de entrada no válida")
)

// invalidByte marca una entrada no válida en la tabla de decodificación.
const invalidByte = 0xFF

// Encoding es un códec base64 con alfabeto propio. Se construye una vez y se
// reutiliza; NewEncoding precalcula la tabla inversa para que decodificar no
// tenga que recorrer el alfabeto por cada carácter.
type Encoding struct {
	enc [64]byte
	dec [256]byte
	pad bool
}

// NewEncoding devuelve un códec sobre alphabet, que DEBE tener exactamente 64
// caracteres ASCII distintos. pad indica si la salida lleva relleno '='.
func NewEncoding(alphabet string, pad bool) (*Encoding, error) {
	if len(alphabet) != 64 {
		return nil, ErrAlphabetLength
	}
	var enc [64]byte
	var dec [256]byte
	for i := range dec {
		dec[i] = invalidByte
	}
	for i := 0; i < 64; i++ {
		c := alphabet[i]
		if c > 127 {
			return nil, ErrAlphabetNonASCII
		}
		if dec[c] != invalidByte {
			return nil, ErrAlphabetDuplicate
		}
		enc[i] = c
		dec[c] = byte(i)
	}
	return &Encoding{enc: enc, dec: dec, pad: pad}, nil
}

// EncodedLen devuelve la longitud en bytes de la codificación de n bytes de entrada.
func (e *Encoding) EncodedLen(n int) int {
	if e.pad {
		return (n + 2) / 3 * 4
	}
	// Sin relleno: 3 bytes -> 4 chars, 1 byte -> 2 chars, 2 bytes -> 3 chars.
	switch n % 3 {
	case 1:
		return n/3*4 + 2
	case 2:
		return n/3*4 + 3
	default:
		return n / 3 * 4
	}
}

// DecodedLen devuelve la longitud máxima en bytes de la decodificación de n
// bytes codificados. Para pad=true asume entrada con relleno; para pad=false
// asume entrada sin relleno.
func (e *Encoding) DecodedLen(n int) int {
	if e.pad {
		return n / 4 * 3
	}
	switch n % 4 {
	case 2:
		return n/4*3 + 1
	case 3:
		return n/4*3 + 2
	default:
		return n / 4 * 3
	}
}

// Encode codifica src con el alfabeto del Encoding.
func (e *Encoding) Encode(src []byte) string {
	if len(src) == 0 {
		return ""
	}
	n := e.EncodedLen(len(src))
	dst := make([]byte, 0, n)
	i := 0
	for ; i+2 < len(src); i += 3 {
		v := uint(src[i])<<16 | uint(src[i+1])<<8 | uint(src[i+2])
		dst = append(dst,
			e.enc[v>>18&0x3F],
			e.enc[v>>12&0x3F],
			e.enc[v>>6&0x3F],
			e.enc[v&0x3F],
		)
	}
	switch len(src) - i {
	case 1:
		v := uint(src[i]) << 16
		dst = append(dst, e.enc[v>>18&0x3F], e.enc[v>>12&0x3F])
		if e.pad {
			dst = append(dst, '=', '=')
		}
	case 2:
		v := uint(src[i])<<16 | uint(src[i+1])<<8
		dst = append(dst, e.enc[v>>18&0x3F], e.enc[v>>12&0x3F], e.enc[v>>6&0x3F])
		if e.pad {
			dst = append(dst, '=')
		}
	}
	return string(dst)
}

// Decode decodifica s con el alfabeto del Encoding. Es estricta: rechaza
// caracteres fuera del alfabeto, longitudes imposibles, relleno incorrecto y
// codificaciones no canónicas (bits sobrantes no cero, RFC 4648 §3.5).
func (e *Encoding) Decode(s string) ([]byte, error) {
	if len(s) == 0 {
		return []byte{}, nil
	}

	var core string
	if e.pad {
		if len(s)%4 != 0 {
			return nil, ErrInvalidLength
		}
		padding := 0
		if s[len(s)-1] == '=' {
			padding = 1
			if len(s) >= 2 && s[len(s)-2] == '=' {
				padding = 2
			}
		}
		core = s[:len(s)-padding]
		switch len(core) % 4 {
		case 0:
			if padding != 0 {
				return nil, ErrInvalidLength
			}
		case 2:
			if padding != 2 {
				return nil, ErrInvalidLength
			}
		case 3:
			if padding != 1 {
				return nil, ErrInvalidLength
			}
		default:
			return nil, ErrInvalidLength
		}
	} else {
		if len(s)%4 == 1 {
			return nil, ErrInvalidLength
		}
		// Sin relleno la entrada no debe contener '='.
		for i := 0; i < len(s); i++ {
			if s[i] == '=' {
				return nil, ErrInvalidCharacter
			}
		}
		core = s
	}

	// Validación y decodificación en un único bucle de bits (buf/bits).
	// Este es el ÚNICO bucle de decodificación del paquete.
	var n int
	switch len(core) % 4 {
	case 2:
		n = len(core)/4*3 + 1
	case 3:
		n = len(core)/4*3 + 2
	default:
		n = len(core) / 4 * 3
	}
	dst := make([]byte, 0, n)
	var buf uint
	var bits uint
	for i := 0; i < len(core); i++ {
		c := e.dec[core[i]]
		if c == invalidByte {
			return nil, ErrInvalidCharacter
		}
		buf = buf<<6 | uint(c)
		bits += 6
		if bits >= 8 {
			bits -= 8
			dst = append(dst, byte(buf>>bits))
		}
	}
	if bits > 0 && buf&(1<<bits-1) != 0 {
		return nil, ErrInvalidCharacter
	}
	return dst, nil
}
