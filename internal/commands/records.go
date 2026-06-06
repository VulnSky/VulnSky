package commands

import (
	"fmt"

	"vulnsky/internal/store"

	"github.com/spf13/cobra"
)

func newRecordsCommand(state *rootState) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "records",
		Short: "Query local operation records",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(recordsUploadsCommand(state))
	cmd.AddCommand(recordsImagesCommand(state))
	cmd.AddCommand(recordsDeploymentsCommand(state))
	return cmd
}

func recordsUploadsCommand(state *rootState) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "uploads",
		Short: "List local OSS upload records",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCommandConfig(state)
			if err != nil {
				return err
			}
			st := store.New(cfg.DBPath)
			if err := st.Init(); err != nil {
				return err
			}
			records, err := st.ListUploads(limit)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ID\tBUCKET\tOBJECT_KEY\tSIZE\tSTATUS\tUPLOADED_AT")
			for _, record := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%d\t%s\t%s\n",
					record.ID,
					record.Bucket,
					record.ObjectKey,
					record.FileSize,
					record.UploadStatus,
					record.UploadedAt,
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum records to show")
	return cmd
}

func recordsImagesCommand(state *rootState) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "images",
		Short: "List local image import records",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCommandConfig(state)
			if err != nil {
				return err
			}
			st := store.New(cfg.DBPath)
			if err := st.Init(); err != nil {
				return err
			}
			records, err := st.ListImageImports(limit)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ID\tIMAGE_ID\tIMAGE_NAME\tOSS_OBJECT\tTASK_ID\tSTATUS\tSTARTED_AT\tFINISHED_AT")
			for _, record := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					record.ID,
					record.ImageID,
					record.ImageName,
					record.OSSObject,
					record.TaskID,
					record.TaskStatus,
					record.StartedAt,
					record.FinishedAt,
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum records to show")
	return cmd
}

func recordsDeploymentsCommand(state *rootState) *cobra.Command {
	var limit int
	cmd := &cobra.Command{
		Use:   "deployments",
		Short: "List local ECS deployment records",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadCommandConfig(state)
			if err != nil {
				return err
			}
			st := store.New(cfg.DBPath)
			if err := st.Init(); err != nil {
				return err
			}
			records, err := st.ListDeployments(limit)
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ID\tINSTANCE_ID\tSOURCE_IMAGE_ID\tPREVIOUS_IMAGE_ID\tDISK_ID\tSTATUS\tSTARTED_AT\tFINISHED_AT")
			for _, record := range records {
				fmt.Fprintf(cmd.OutOrStdout(), "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					record.ID,
					record.InstanceID,
					record.SourceImageID,
					record.PreviousImageID,
					record.NewDiskID,
					record.Status,
					record.StartedAt,
					record.FinishedAt,
				)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 50, "maximum records to show")
	return cmd
}
