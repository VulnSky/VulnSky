package commands

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"vulnsky/internal/store"
	"vulnsky/internal/util"

	"github.com/spf13/cobra"
)

func newOSSCommand(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "oss",
		Short: "Manage OSS objects",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "ls [prefix]",
		Short: "List OSS objects",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			prefix := ""
			if len(args) > 0 {
				prefix = args[0]
			}
			return runOSSList(cmd, state, prefix)
		},
	})

	uploadCmd := &cobra.Command{
		Use:   "upload <qcow2-path>",
		Short: "Upload a QCOW2 file to OSS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, _ := cmd.Flags().GetString("key")
			return runOSSUpload(cmd, state, args[0], key)
		},
	}
	uploadCmd.Flags().String("key", "", "OSS object key")
	cmd.AddCommand(uploadCmd)

	linkCmd := &cobra.Command{
		Use:   "link <object-key>",
		Short: "Create a presigned OSS download URL",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			expiresValue, _ := cmd.Flags().GetString("expires")
			expires, err := time.ParseDuration(expiresValue)
			if err != nil {
				return fmt.Errorf("invalid expires duration %q: %w", expiresValue, err)
			}
			return runOSSLink(cmd, state, args[0], expires)
		},
	}
	linkCmd.Flags().String("expires", "6h", "presigned URL duration")
	cmd.AddCommand(linkCmd)

	return cmd
}

func runOSSList(cmd *cobra.Command, state *rootState, prefix string) error {
	cfg, client, err := loadOSSClient(state)
	if err != nil {
		return err
	}
	objects, err := client.ListObjects(cmd.Context(), cfg.OSSBucket, prefix)
	if err != nil {
		return err
	}
	for _, object := range objects {
		fmt.Fprintln(cmd.OutOrStdout(), object)
	}
	return nil
}

func runOSSLink(cmd *cobra.Command, state *rootState, objectKey string, expires time.Duration) error {
	cfg, client, err := loadOSSClient(state)
	if err != nil {
		return err
	}
	url, err := client.PresignGet(cmd.Context(), cfg.OSSBucket, objectKey, expires)
	if err != nil {
		return err
	}
	fmt.Fprintln(cmd.OutOrStdout(), url)
	return nil
}

func runOSSUpload(cmd *cobra.Command, state *rootState, localPath string, objectKey string) error {
	cfg, ossClient, err := loadOSSClient(state)
	if err != nil {
		return err
	}
	stsClient, err := newOSSSTSClient(state, cfg)
	if err != nil {
		return err
	}
	accountID, _, err := stsClient.GetCallerIdentity(cmd.Context())
	if err != nil {
		return err
	}

	sum, size, err := util.FileSHA256(localPath)
	if err != nil {
		return err
	}
	if objectKey == "" {
		objectKey = defaultObjectKey(cfgDefaultObjectPrefix(cfg.DefaultObjectPrefix), localPath)
	}

	st := store.New(cfg.DBPath)
	if err := st.Init(); err != nil {
		return err
	}
	if existing, err := st.FindUploadedBySHA256(cfg.ProfileName, accountID, cfg.OSSRegionID, sum); err != nil {
		return err
	} else if existing != nil {
		exists, err := ossClient.ObjectExists(cmd.Context(), existing.Bucket, existing.ObjectKey)
		if err != nil {
			return err
		}
		if exists {
			fmt.Fprintf(cmd.OutOrStdout(), "reused %s sha256=%s\n", existing.ObjectKey, sum)
			return nil
		}
	}

	requestID, err := ossClient.UploadFile(cmd.Context(), cfg.OSSBucket, objectKey, localPath, nil)
	if err != nil {
		return err
	}
	_, err = st.InsertUpload(store.UploadRecord{
		ProfileName:  cfg.ProfileName,
		AccountID:    accountID,
		RegionID:     cfg.OSSRegionID,
		OSSRegionID:  cfg.OSSRegionID,
		Bucket:       cfg.OSSBucket,
		ObjectKey:    objectKey,
		LocalPath:    localPath,
		FileName:     filepath.Base(localPath),
		FileSize:     size,
		SHA256:       sum,
		UploadStatus: "uploaded",
		RequestID:    requestID,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(cmd.OutOrStdout(), "uploaded %s sha256=%s size=%d\n", objectKey, sum, size)
	return nil
}

func cfgDefaultObjectPrefix(prefix string) string {
	if prefix == "" {
		return "qcow2/"
	}
	return prefix
}

func defaultObjectKey(prefix string, localPath string) string {
	prefix = strings.Trim(prefix, "/")
	if prefix == "" {
		return filepath.Base(localPath)
	}
	return prefix + "/" + filepath.Base(localPath)
}
