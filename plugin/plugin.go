package plugin

import (
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"unsafe"

	"github.com/cloudfoundry/dropsonde"
)

// #cgo LDFLAGS: -lc

/*
#include <arpa/inet.h>
#include <stdio.h>
#include <ulogd/ulogd.h>

void ulogd_register_plugin(struct ulogd_plugin *me) __attribute__((weak));
int config_parse_file(const char *section, struct config_keyset *kset) __attribute__((weak));
const char *inet_ntop(int, const void *, char *, socklen_t) __attribute__((weak));

static inline int isValid(struct ulogd_key *keys, unsigned int kidx) {
	        return pp_is_valid(keys, kidx);
}

static inline int8_t ikey_get_i8(struct ulogd_key *key)
{
	return key->u.source->u.value.i8;
}

static inline int16_t ikey_get_i16(struct ulogd_key *key)
{
	return key->u.source->u.value.i16;
}

static inline int32_t ikey_get_i32(struct ulogd_key *key)
{
	return key->u.source->u.value.i32;
}

static inline int64_t ikey_get_i64(struct ulogd_key *key)
{
	return key->u.source->u.value.i64;
}

char *configString(struct ulogd_pluginstance *pi, unsigned int k);
int configInt(struct ulogd_pluginstance *pi, unsigned int k);

static inline const char *ikey_get_ip4(struct ulogd_key *key) {
	char *retVal = malloc(INET_ADDRSTRLEN);
	if (retVal) {
		bzero(retVal, INET_ADDRSTRLEN);
		uint32_t u32 = ikey_get_u32(key);
		inet_ntop(AF_INET, &u32, retVal, INET_ADDRSTRLEN - 1);
	}
	return retVal;
}

static inline const char *ikey_get_ip6(struct ulogd_key *key) {
	char *retVal = malloc(INET6_ADDRSTRLEN);
	if (retVal) {
		bzero(retVal, INET6_ADDRSTRLEN);
		inet_ntop(AF_INET6, ikey_get_u128(key), retVal, INET6_ADDRSTRLEN - 1);
	}
	return retVal;
}

static struct ulogd_key plugin_inp_additional[] = {
	// keys come from IP2INSTANCE plugin
	{
		.type = ULOGD_RET_STRING,
		.name = "cf.sinstance",
	},
	{
		.type = ULOGD_RET_STRING,
		.name = "cf.dinstance",
	},
};

static struct ulogd_key *find_named_okey(char const *name, struct ulogd_pluginstance_stack *ps) {
       struct ulogd_key *result = NULL;

       int i;
       for (i = 0; i < ARRAY_SIZE(plugin_inp_additional); i++) {
               if (0 == strcmp(name, plugin_inp_additional[i].name)) {
                       free((void *)name);
                       return &plugin_inp_additional[i];
               }
       }

       if (ps) {
               struct ulogd_pluginstance *pi;
               llist_for_each_entry(pi, &ps->list, list) {
                       int i;
                       for (i = 0; i < pi->plugin->output.num_keys; i++) {
                               if (0 == strcmp(name, pi->plugin->output.keys[i].name)) {
                                       free((void *)name);
                                       return &pi->plugin->output.keys[i];
                               }
                       }
               }
       }

       free((void *)name);
       return NULL;
}

*/
import "C"

func configTuple(pi *C.struct_ulogd_pluginstance) (destination, origin, sender, instance string, format []string) {
	destination = C.GoString(C.configString(pi, 0))
	origin = C.GoString(C.configString(pi, 1))
	sender = C.GoString(C.configString(pi, 2))
	instance = fmt.Sprintf("%d", C.configInt(pi, 3))

	// everything else is a format string, because individual format strings are
	// too limited in length to be useful as templates
	for i := C.uint(4); i < (*pi).config_kset.num_ces; i++ {
		format = append(format, C.GoString(C.configString(pi, i)))
	}

	return
}

type nilWriter struct{}

func (w *nilWriter) Write(b []byte) (int, error) {
	return len(b), nil
}

// int (*configure)(struct ulogd_pluginstance *instance,
//                  struct ulogd_pluginstance_stack *stack)
//export configurePlugin
func configurePlugin(pi *C.struct_ulogd_pluginstance, ps *C.struct_ulogd_pluginstance_stack) C.int {
	log.Printf(">>> configurePlugin dropsonde pi=%x", unsafe.Pointer(pi))
	defer log.Printf("configurePlugin dropsonde pi=%x <<<", unsafe.Pointer(pi))

	parseResult := C.config_parse_file(&(pi.id[0]), pi.config_kset)
	if 0 != parseResult {
		log.Printf("bailing because config_parse_file returned %d", parseResult)
		return -1
	}

	destination, _, _, _, format := configTuple(pi)

	// ensure that the `destination` is parseable as a host:port pair
	log.Printf("pluginstance config got destination=%q", destination)
	_, _, err := net.SplitHostPort(destination)
	if nil != err {
		log.Printf("failed to split host:port of %q: %v", destination, err)
		return -1
	}

	log.Printf("pluginstance config got format=%q", format[0])

	fmtKeys := map[string]*C.struct_ulogd_key{}
	for _, fmtConfig := range format {
		for _, m := range regexp.MustCompile(`ikey "([^"]*)"`).FindAllStringSubmatch(fmtConfig, -1) {
			msgKey := m[1]
			if fmtKeys[msgKey] == nil {
				okey := C.find_named_okey(C.CString(msgKey), ps)
				if okey == nil {
					log.Printf("`format` refers to okey=%q, but no stack plugin provides that.", msgKey)
					return -1
				}

				log.Printf("found input key %q in pluginstance `format`", msgKey)
				fmtKeys[msgKey] = okey
			}
		}
	}

	if (*pi).input.keys != nil {
		C.free(unsafe.Pointer((*pi).input.keys))
	}

	numKeys := C.size_t(2 + len(fmtKeys))
	(*pi).input.num_keys = C.uint(numKeys)
	(*pi).input._type = C.ULOGD_DTYPE_PACKET | C.ULOGD_DTYPE_FLOW
	(*pi).input.keys = (*C.struct_ulogd_key)(C.malloc(numKeys * C.sizeof_struct_ulogd_key))

	if (*pi).input.keys == nil {
		log.Printf("allocating space for %d keys: ENOMEM", numKeys)
		return -1
	}

	log.Printf("allocated space for %d keys", numKeys)

	var key *C.struct_ulogd_key

	log.Printf("copying ikey for cf.sinstance")
	key = C.find_named_okey(C.CString("cf.sinstance"), ps)
	if key == nil {
		log.Printf("To use the DSONDE plugin, you must also configure an IP2INSTANCE plugin.")
		return -1
	}
	C.memcpy(unsafe.Pointer(nthKey(0, &(*pi).input)), unsafe.Pointer(key), C.sizeof_struct_ulogd_key)

	log.Printf("copying ikey for cf.dinstance")
	key = C.find_named_okey(C.CString("cf.dinstance"), ps)
	if key == nil {
		log.Printf("Can't happen: you have cf.sinstance, but not cf.dinstance?")
		return -1
	}
	C.memcpy(unsafe.Pointer(nthKey(1, &(*pi).input)), unsafe.Pointer(key), C.sizeof_struct_ulogd_key)

	var i C.uint = 2 // 0, 1 were cf.sinstance, cf.dinstance
	for kName, key := range fmtKeys {
		log.Printf("copying ikey for %q (%q)", kName, C.GoString(&(*key).name[0]))
		C.memcpy(unsafe.Pointer(nthKey(i, &(*pi).input)), unsafe.Pointer(key), C.sizeof_struct_ulogd_key)
		i++
	}

	return 0
}

// int (*start)(struct ulogd_pluginstance *pi)
//export startPlugin
func startPlugin(pi *C.struct_ulogd_pluginstance) C.int {
	log.Printf(">>> startPlugin dropsonde pi=%x", unsafe.Pointer(pi))
	defer log.Printf("startPlugin dropsonde pi=%x <<<", unsafe.Pointer(pi))

	destination, origin, sender, instance, _ := configTuple(pi)
	dropsonde.Initialize(destination, origin, sender, instance)

	return 0
}

const (
	kNone  = 0x0000
	kInt8  = 0x0001
	kInt16 = 0x0002
	kInt32 = 0x0003
	kInt64 = 0x0004

	kUint8  = 0x0011
	kUint16 = 0x0012
	kUint32 = 0x0013
	kUint64 = 0x0014

	kBool = 0x0050

	kIpAddr  = 0x0100
	kIp6Addr = 0x0200

	kString = 0x8020
	kRaw    = 0x8030
	kRawStr = 0x8040
)

// int (*interp)(struct ulogd_pluginstance *instance)
//export doOutput
func doOutput(pi *C.struct_ulogd_pluginstance) C.int {
	// log.Printf(">>> doOutput dropsonde pi=%x", unsafe.Pointer(pi))
	// defer log.Printf("doOutput dropsonde pi=%x <<<", unsafe.Pointer(pi))

	if C.isValid((*pi).input.keys, 0)+C.isValid((*pi).input.keys, 1) > 0 {
		_, _, sender, _, format := configTuple(pi)

		ikeys := map[string]string{}
		var v string
		for i := C.uint(0); i < (*pi).input.num_keys; i++ {
			if 0 != C.isValid((*pi).input.keys, i) {
				ikey := nthKey(i, &(*pi).input)
				switch uint(ikey._type) {
				case kNone:
					v = "<void>"
				case kInt8:
					v = strconv.FormatInt(int64(C.ikey_get_i8(ikey)), 10)
				case kInt16:
					v = strconv.FormatInt(int64(C.ikey_get_i16(ikey)), 10)
				case kInt32:
					v = strconv.FormatInt(int64(C.ikey_get_i32(ikey)), 10)
				case kInt64:
					v = strconv.FormatInt(int64(C.ikey_get_i64(ikey)), 10)
				case kUint8:
					v = strconv.FormatUint(uint64(C.ikey_get_u8(ikey)), 10)
				case kUint16:
					v = strconv.FormatUint(uint64(C.ikey_get_u16(ikey)), 10)
				case kUint32:
					v = strconv.FormatUint(uint64(C.ikey_get_u32(ikey)), 10)
				case kUint64:
					v = strconv.FormatUint(uint64(C.ikey_get_u64(ikey)), 10)
				case kBool:
					switch uint(C.ikey_get_u8(ikey)) {
					case 0:
						v = "false"
					default:
						v = "true"
					}
				case kIpAddr:
					c, err := C.ikey_get_ip4(ikey)
					if c == nil {
						log.Printf("converting an IP4 address: %v", err)
						return -1
					}

					v = C.GoString(c)
					C.free(unsafe.Pointer(c))
				case kIp6Addr:
					c, err := C.ikey_get_ip6(ikey)
					if c == nil {
						log.Printf("converting an IP6 address: %v", err)
						return -1
					}

					v = C.GoString(c)
					C.free(unsafe.Pointer(c))
				case kString:
					v = C.GoString((*C.char)(C.ikey_get_ptr(ikey)))
				case kRaw:
					fallthrough
				case kRawStr:
					v = "<raw>"
				}
				kName := C.GoString(&ikey.name[0])
				// log.Printf("caching ikey %q=%q", kName, v)
				ikeys[kName] = v
			} else {
				// log.Printf("cowardly refusing to cache invalid ikey at index=%d", i)
			}
		}

		dw := MakeDsondeWriter()

		t := template.New("logMessage").Funcs(template.FuncMap{"ikey": func(kName string) (value string, err error) {
			value = ikeys[kName]
			// log.Printf("executing template, found %q=%q", kName, value)
			return value, nil
		}, "dsonde": func(mode string) (value string, err error) {
			switch mode {
			case "err":
				dw.mode = appErr
			case "out":
				dw.mode = appOut
			case "nothing":
				dw.mode = appNone
			default:
				return "", fmt.Errorf("unknown dropsonde mode %q", mode)
			}

			// log.Printf("executing template, dsonde mode was set to %v", dw.mode)
			return "", nil
		}})

		var err error
		for i, f := range format[1:] {
			key := fmt.Sprintf("f%d", i+1)
			t, err = t.Parse(fmt.Sprintf("{{define %q}}%s{{end}}", key, f))
			if err != nil {
				log.Printf("parsing template %q: %v", f, err)
				return -1
			}
		}

		t, err = t.Parse(format[0])

		if err != nil {
			log.Printf("parsing template %q: %v", format[0], err)
			return -1
		}

		sinstance := ikeys["cf.sinstance"]
		if sinstance != "" {
			// log.Printf("DSONDE using sinstance=%q", sinstance)
			guid, idx, err := guidAndIdx(sinstance)
			if nil != err {
				log.Printf("%v", err)
				return -1
			}

			dw.guid = guid
			dw.sender = sender
			dw.instance = uint(idx)

			// log.Printf("will execute template format=%q", format[0])
			err = t.Execute(dw, nil)
			dw.Flush()
			if err != nil {
				log.Printf("executing `format` template to write to dropsonde: %v", err)
				return -1
			}
		}

		dinstance := ikeys["cf.dinstance"]
		if dinstance != "" {
			// log.Printf("DSONDE using dinstance=%q", sinstance)
			guid, idx, err := guidAndIdx(dinstance)
			if nil != err {
				log.Printf("%v", err)
				return -1
			}

			dw.guid = guid
			dw.sender = sender
			dw.instance = uint(idx)

			// log.Printf("will execute template format=%q", format[0])
			err = t.Execute(dw, nil)
			dw.Flush()
			if err != nil {
				log.Printf("executing `format` template to write to dropsonde: %v", err)
				return -1
			}
		}
	}

	return 0
}

// int (*stop)(struct ulogd_pluginstance *pi)
//export stopPlugin
func stopPlugin(pi *C.struct_ulogd_pluginstance) C.int {
	log.Printf(">>> stopPlugin pi=%x", unsafe.Pointer(pi))
	defer log.Printf("stopPlugin pi=%x <<<", unsafe.Pointer(pi))

	return 0
}

func nthKey(n C.uint, keyset *C.struct_ulogd_keyset) *C.struct_ulogd_key {
	if n >= (*keyset).num_keys {
		panic(fmt.Errorf("index (%d) out of range for keyset.num_keys=(%d)", n, (*keyset).num_keys))
	}

	sz := (C.uint)(C.sizeof_struct_ulogd_key)
	k0 := unsafe.Pointer((*keyset).keys)
	pk := (*C.struct_ulogd_key)(unsafe.Pointer(uintptr(k0) + uintptr(n*sz)))

	return pk
}

func guidAndIdx(instanceStr string) (guid string, idx int, err error) {
	gi := strings.Split(instanceStr, "/")

	guid = gi[0]
	uidx, err := strconv.ParseUint(gi[1], 10, 32)
	if err != nil {
		return "", -1, err
	}

	return guid, int(uidx), nil
}
