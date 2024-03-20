package main

import (
	"os"
	"strconv"

	"github.com/posener/complete/v2"
	"github.com/posener/complete/v2/predict"
)

var (
	predictAnyFileAndDir = predict.Files("*")
	completer            = CreateCompleter(func(c *Completer) *complete.Command {
		return &complete.Command{
			Sub: map[string]*complete.Command{
				SHELL_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"c": predict.Files("*.ix"),
					},
				},
				EVAL_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"c": predict.Files("*.ix"),
					},
				},
				EVAL_ALIAS_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"c": predict.Files("*.ix"),
					},
				},
				CHECK_SUBCMD: {},
				HELP_SUBCMD:  {},
				RUN_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"test":                     complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"test-trusted":             complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"fully-trusted":            complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"show-bytecode":            complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"no-optimization":          complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"allow-browser-automation": complete.PredictFunc(c.predictFileOrDirAfterSwitch),
						"t":                        complete.PredictFunc(c.predictFileOrDirAfterSwitch),
					},
					Args: predict.Files("*"),
				},
				ADD_SERVICE_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"inox-cloud":               predict.Nothing,
						"tunnel-provider":          predict.Set{"cloudflare"},
						"expose-project-servers":   predict.Nothing,
						"expose-wev-servers":       predict.Nothing,
						"allow-browser-automation": predict.Nothing,
					},
				},
				REMOVE_SERVICE_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"remove-tunnel-configs":  predict.Nothing,
						"remove-inoxd-user":      predict.Nothing,
						"remove-inoxd-homedir":   predict.Nothing,
						"remove-env-file":        predict.Nothing,
						"remove-data-dir":        predict.Nothing,
						"dangerously-remove-all": predict.Nothing,
					},
				},
				UPGRADE_INOX_SUBCMD: {},
				PROJECT_SERVER_SUBCMD: {
					Flags: map[string]complete.Predictor{
						"config": predict.Set{`'{"port":8305}'`},
					},
				},
				INSTALL_COMPLETIONS_SUBCMD:   {},
				UNINSTALL_COMPLETIONS_SUBCMD: {},
			},
		}
	})
)

type Completer struct {
	*complete.Command
	currentCompLine  string
	currentCompPoint int //-1 if not retrieved
}

func CreateCompleter(create func(c *Completer) *complete.Command) *Completer {
	c := &Completer{}
	c.Command = create(c)
	return c
}

func (c *Completer) Complete(name string) {
	c.currentCompLine = os.Getenv("COMP_LINE")
	c.currentCompPoint, _ = strconv.Atoi(os.Getenv("COMP_POINT")) //ignore error because .CommandComplete will also check the value

	if c.currentCompPoint > len(c.currentCompLine) {
		c.currentCompPoint = len(c.currentCompLine)
	}

	c.Command.Complete(name)
}

func (c *Completer) beforeCursorPoint() string {
	return c.currentCompLine[:c.currentCompPoint]
}

func (c *Completer) predictFileOrDirAfterSwitch(prefix string) (results []string) {
	s := c.beforeCursorPoint()
	if s == "" {
		return
	}

	switch s[len(s)-1] {
	case '=':
		//The flag is a switch, it does not accept any value.
		return
	default:
		return predictAnyFileAndDir.Predict(prefix)
	}
}
