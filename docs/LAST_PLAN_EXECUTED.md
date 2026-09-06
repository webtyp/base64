---
PLAN: "feat: codificador base64 con alfabeto configurable"
EXECUTOR: jules
REVIEWER: none
---

> Este plan se despacha con el flujo CodeJob. Ver skill: agents-workflow.

# Plan — `webtyp/base64`: alfabeto configurable

## Contexto

Este paquete existe **por tamaño**, no por compatibilidad: `encoding/base64`
funciona con TinyGo, pero arrastra mucho más de lo que la tarea necesita. Hoy
expone dos variantes fijas de RFC 4648 (`Encode`/`Decode` y
`URLEncode`/`URLDecode`) y **no tiene ninguna dependencia** — ni de la stdlib ni
de `webtyp/*`. Eso se conserva.

Hace falta una tercera forma: **alfabeto propio y sin relleno**. La pide bcrypt,
que codifica sal y hash con `./ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789`
y sin `=` al final. Se implementa aquí, y no dentro de bcrypt, porque una tercera
tabla base64 copiada en otro repositorio es exactamente la duplicación que este
paquete existe para evitar.

## Qué construir

Un archivo nuevo, `alphabet.go`:

```go
// Encoding es un códec base64 con alfabeto propio. Se construye una vez y se
// reutiliza; NewEncoding precalcula la tabla inversa para que decodificar no
// tenga que recorrer el alfabeto por cada carácter.
type Encoding struct {
	enc [64]byte
	dec [256]byte // 0xFF = carácter no válido
	pad bool
}

// NewEncoding devuelve un códec sobre alphabet, que DEBE tener exactamente 64
// caracteres ASCII distintos. pad indica si la salida lleva relleno '='.
func NewEncoding(alphabet string, pad bool) (*Encoding, error)

func (e *Encoding) Encode(src []byte) string
func (e *Encoding) Decode(s string) ([]byte, error)
func (e *Encoding) EncodedLen(n int) int
func (e *Encoding) DecodedLen(n int) int
```

**Reutiliza el motor que ya existe.** `Encode`/`Decode` y
`URLEncode`/`URLDecode` de este paquete ya hacen el desplazamiento de bits; lo
único que cambia entre las tres formas es la tabla y el relleno. Refactoriza el
núcleo a funciones internas que reciban la tabla, y deja las cuatro funciones
públicas actuales como envoltorios sobre dos `*Encoding` de paquete construidos
en `init()`. **No dupliques el bucle de bits tres veces.**

Las cuatro funciones públicas de hoy **no cambian de firma ni de comportamiento**:
es un añadido, no una ruptura.

## Errores

Sin literales sueltos. Constantes exportadas, y el paquete sigue **sin importar
nada** — ni `webtyp/fmt`. Declara un tipo de error propio:

```go
type Error string

func (e Error) Error() string { return string(e) }

const (
	ErrAlphabetLength   = Error("base64: el alfabeto debe tener exactamente 64 caracteres")
	ErrAlphabetDuplicate = Error("base64: el alfabeto tiene caracteres repetidos")
	ErrAlphabetNonASCII = Error("base64: el alfabeto sólo admite ASCII")
	ErrInvalidCharacter = Error("base64: carácter no válido en la entrada")
	ErrInvalidLength    = Error("base64: longitud de entrada no válida")
)
```

**Anti-footgun:** este paquete presume de cero dependencias y el README lo
declara. Importar `webtyp/fmt` para construir un error rompería esa promesa por
comodidad. `type Error string` cuesta cero bytes de dependencia.

## Tests

En `alphabet_test.go`, y siguiendo el patrón de los tests que ya existen (hay un
par WASM/nativo: `RunBase64Tests` compartido). Casos obligatorios:

| Caso | Espera |
|---|---|
| alfabeto de 63 o 65 caracteres | `ErrAlphabetLength` |
| alfabeto con un carácter repetido | `ErrAlphabetDuplicate` |
| alfabeto con un byte > 127 | `ErrAlphabetNonASCII` |
| ida y vuelta con el alfabeto de bcrypt, entradas de 0 a 64 bytes | idéntico al original |
| `pad=false`, entrada de 1 y 2 bytes | salida **sin** `=` |
| `pad=true` con el alfabeto estándar | idéntico a `Encode` de este paquete |
| decodificar una cadena con un carácter fuera del alfabeto | `ErrInvalidCharacter` |
| decodificar longitud imposible (p. ej. 1 carácter, sin relleno) | `ErrInvalidLength` |

El caso del alfabeto estándar con `pad=true` es el que demuestra que el
refactor no cambió el motor: tiene que dar **exactamente** lo mismo que
`Encode`.

## Criterios de aceptación

- [ ] `go list -f '{{join .Imports " "}}' .` → **vacío**. El paquete sigue sin
      dependencias.
- [ ] `Encode`, `Decode`, `URLEncode` y `URLDecode` conservan firma y
      comportamiento; sus tests actuales pasan sin tocarlos.
- [ ] `grep -c "for.*shift\|<< 18\|>> 18" *.go` — el bucle de bits aparece **una
      sola vez**, no una por variante.
- [ ] Todos los casos de la tabla, en verde.
- [ ] `README.md` documenta `NewEncoding` con el ejemplo del alfabeto de bcrypt.

## Fuera de alcance

Streaming (`io.Reader`/`io.Writer`), base32, base58. Nada de eso se construye
hasta que alguien lo pida — y `io` en particular **no se importa nunca aquí**.
