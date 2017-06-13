#include "_cgo_export.h"

#define DROPSONDE_DESTINATION_LOCALHOST "localhost:3457"

static struct config_keyset c = {
	.ces = {
		{
			.key = "destination",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_NONE,
			.u = { .string = DROPSONDE_DESTINATION_LOCALHOST },
		}, {
			.key = "origin",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_MANDATORY,
		}, {
			.key = "sender",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_MANDATORY,
		}, {
			.key = "instance",
			.type = CONFIG_TYPE_INT,
			.options = CONFIG_OPT_NONE,
			.u = { .value = 0 },
		}, {
			.key = "format",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_MANDATORY,
		}, {
			.key = "f1",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_NONE,
		}, {
			.key = "f2",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_NONE,
		}, {
			.key = "f3",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_NONE,
		}, {
			.key = "f4",
			.type = CONFIG_TYPE_STRING,
			.options = CONFIG_OPT_NONE,
		},
	},
	.num_ces = 9,
};

static struct ulogd_plugin p = {
	.name = "DSONDE",
	.output = {
		.type = ULOGD_DTYPE_SINK,
	},
	.config_kset = &c,
	.priv_size = 0,
	.configure = &configurePlugin,
	.start = &startPlugin,
	.stop = &stopPlugin,
	.interp = &doOutput,
	.version = VERSION,
};

void __attribute__ ((constructor)) init(void);
void init(void) {
	ulogd_register_plugin(&p);
}

