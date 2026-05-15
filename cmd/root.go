package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/chirs/stordiag/internal/driver"
)

var (
	cfgFile    string
	endpoint   string
	ak         string
	sk         string
	bucket     string
	secure     bool
	jsonOut    bool
	timeoutSec int

	version = "dev"

	rootCmd = &cobra.Command{
		Use:   "stordiag",
		Short: "Storage Diagnostics Toolkit",
		Long: `stordiag - distributed storage debugging toolbox.
Supports S3-compatible object storage and POSIX filesystems.`,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Name() == "version" || cmd.Name() == "help" {
				return nil
			}
			return initDriver()
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
)

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

var globalDriver driver.Driver

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&endpoint, "endpoint", "", "Storage endpoint (host:port or path for POSIX)")
	rootCmd.PersistentFlags().StringVar(&ak, "access-key", "", "Access key")
	rootCmd.PersistentFlags().StringVar(&sk, "secret-key", "", "Secret key")
	rootCmd.PersistentFlags().StringVar(&bucket, "bucket", "", "Bucket name")
	rootCmd.PersistentFlags().BoolVar(&secure, "secure", false, "Use TLS")
	rootCmd.PersistentFlags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	rootCmd.PersistentFlags().IntVar(&timeoutSec, "timeout", 30, "Operation timeout in seconds")
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "Config file (.stordiag.yaml)")

	_ = viper.BindPFlag("endpoint", rootCmd.PersistentFlags().Lookup("endpoint"))
	_ = viper.BindPFlag("access-key", rootCmd.PersistentFlags().Lookup("access-key"))
	_ = viper.BindPFlag("secret-key", rootCmd.PersistentFlags().Lookup("secret-key"))
	_ = viper.BindPFlag("bucket", rootCmd.PersistentFlags().Lookup("bucket"))
}

func initConfig() {
	if cfgFile != "" {
		viper.SetConfigFile(cfgFile)
	} else {
		viper.AddConfigPath(".")
		viper.SetConfigName(".stordiag")
	}
	viper.SetEnvPrefix("STORDIAG")
	viper.AutomaticEnv()

	_ = viper.ReadInConfig()
}

func initDriver() error {
	ep := viper.GetString("endpoint")
	if ep == "" {
		ep = "localhost:9000"
	}

	ak = viper.GetString("access-key")
	sk = viper.GetString("secret-key")
	bucket = viper.GetString("bucket")
	secure = viper.GetBool("secure")

	// detect driver type: if endpoint looks like a path, use POSIX
	if isPath(ep) {
		d, err := driver.NewPOSIXDriver(ep)
		if err != nil {
			return fmt.Errorf("init posix driver: %w", err)
		}
		globalDriver = d
		fmt.Fprintf(os.Stderr, "driver: posix @ %s\n", ep)
		return nil
	}

	if ak == "" {
		ak = "minioadmin"
	}
	if sk == "" {
		sk = "minioadmin"
	}
	if bucket == "" {
		bucket = "stordiag"
	}

	d, err := driver.NewS3Driver(ep, ak, sk, bucket, secure)
	if err != nil {
		return fmt.Errorf("init s3 driver: %w", err)
	}
	globalDriver = d
	fmt.Fprintf(os.Stderr, "driver: s3 @ %s/%s\n", ep, bucket)
	return nil
}

func isPath(s string) bool {
	return s[0] == '/' || s[0] == '.'
}

func ctx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(timeoutSec)*time.Second)
}
