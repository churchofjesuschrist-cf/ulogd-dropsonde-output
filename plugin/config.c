#include <ulogd/ulogd.h>

#include <regex.h>
#include <sys/errno.h>
#include <stdio.h>

static regex_t *re = NULL;
char const *const MATCH_STR = "\\\\x22";
char const REPL_CHR = '\"';
void fixupConfigString(char *cs) {
	char regerr[100];
	int e = 0;
	if (!re) {
		if ((re = malloc(sizeof(regex_t)))) {
			e = regcomp(re, MATCH_STR, 0);
			if (e) {
				regerror(e, re, regerr, 100);
				dprintf(2, "error compiling regex: %s\n", regerr);
				free(re);
				re = NULL;
			}
		} else {
			dprintf(2, "failed to malloc regex: %s\n", strerror(errno));
			return;
		}
	}

	if (!re) {
		dprintf(2, "failed to initialize regex, skipping regexec\n");
		return;
	} else {
		regmatch_t pmatch[1];
		e = regexec(re, cs, 1, pmatch, 0);
		while (!e) {
			*(cs + pmatch[0].rm_so) = REPL_CHR;
			memmove(cs + pmatch[0].rm_so + 1, cs + pmatch[0].rm_eo, 1 + strlen(cs + pmatch[0].rm_eo));
			cs += pmatch[0].rm_so + 1;
			e = regexec(re, cs, 1, pmatch, 0);
		}

		if (e && e != REG_NOMATCH) {
			regerror(e, re, regerr, 100);
			dprintf(2, "error matching regex: %s\n", regerr);
			// deliberately leak re; it is stateless and can be reused,
			// and we get to avoid malloc/initialization in the future.
			return;
		}
	}
}

char *configString(struct ulogd_pluginstance *pi, unsigned int k) {
	if (k >= pi->config_kset->num_ces) return NULL;
	char *result = pi->config_kset->ces[k].u.string;
	fixupConfigString(result);
	return result;
}

int configInt(struct ulogd_pluginstance *pi, unsigned int k) {
	if (k >= pi->config_kset->num_ces) return -1;
	return pi->config_kset->ces[k].u.value;
}
