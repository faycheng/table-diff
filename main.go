package main

import (
	"github.com/spf13/cobra"
)

var conf string

var diffCmd = &cobra.Command{
	Use:   "table-diff",
	Short: "table-diff is a powerful and simple tool for finding difference between MySQL tables efficiently",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		cfg, err := readConf(conf)
		if err != nil {
			log.Fatalf("[table-diff] read configurations:%s", err)
		}
		log.Debugf("[table-diff] good configuration file:%+v", *cfg)
		differ, err := newDiffer(cfg)
		if err != nil {
			log.Fatalf("[table-diff] initiate differ:%s", err)
		}
		err = differ.diff()
		if err != nil {
			log.Fatalf("[table-diff] some fatal exception raises when this app is trying to compare differences:%s", err)
		}
		log.Infof("[table-diff] the validation for comparing differences has finished, you can continue to review the stdout or the file which contains all differences, file:%s", cfg.Output.File)
	},
}

func init() {
	diffCmd.PersistentFlags().StringVar(&conf, "conf", "diff.toml", "config file (default is $PWD/diff.toml)")
}

func main() {
	if err := diffCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
