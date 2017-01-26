package dedupe

import (
	"log"

	"cmd"
	"fs"
	"github.com/spf13/cobra"
)

var (
	dedupeMode = fs.DeduplicateInteractive
)

func init() {
	cmd.Root.AddCommand(commandDefintion)
	commandDefintion.Flags().VarP(&dedupeMode, "dedupe-mode", "", "Dedupe mode interactive|skip|first|newest|oldest|rename.")
}

var commandDefintion = &cobra.Command{
	Use:   "dedupe [mode] remote:path",
	Short: `Interactively find duplicate files delete/rename them.`,
	Long: `
By default ` + "`" + `dedup` + "`" + ` interactively finds duplicate files and offers to
delete all but one or rename them to be different. Only useful with
Google Drive which can have duplicate file names.

The ` + "`" + `dedupe` + "`" + ` command will delete all but one of any identical (same
md5sum) files it finds without confirmation.  This means that for most
duplicated files the ` + "`" + `dedupe` + "`" + ` command will not be interactive.  You
can use ` + "`" + `--dry-run` + "`" + ` to see what would happen without doing anything.

Here is an example run.

Before - with duplicates

    $ rclone lsl drive:dupes
      6048320 2016-03-05 16:23:16.798000000 one.txt
      6048320 2016-03-05 16:23:11.775000000 one.txt
       564374 2016-03-05 16:23:06.731000000 one.txt
      6048320 2016-03-05 16:18:26.092000000 one.txt
      6048320 2016-03-05 16:22:46.185000000 two.txt
      1744073 2016-03-05 16:22:38.104000000 two.txt
       564374 2016-03-05 16:22:52.118000000 two.txt

Now the ` + "`" + `dedupe` + "`" + ` session

    $ rclone dedupe drive:dupes
    2016/03/05 16:24:37 Google drive root 'dupes': Looking for duplicates using interactive mode.
    one.txt: Found 4 duplicates - deleting identical copies
    one.txt: Deleting 2/3 identical duplicates (md5sum "1eedaa9fe86fd4b8632e2ac549403b36")
    one.txt: 2 duplicates remain
      1:      6048320 bytes, 2016-03-05 16:23:16.798000000, md5sum 1eedaa9fe86fd4b8632e2ac549403b36
      2:       564374 bytes, 2016-03-05 16:23:06.731000000, md5sum 7594e7dc9fc28f727c42ee3e0749de81
    s) Skip and do nothing
    k) Keep just one (choose which in next step)
    r) Rename all to be different (by changing file.jpg to file-1.jpg)
    s/k/r> k
    Enter the number of the file to keep> 1
    one.txt: Deleted 1 extra copies
    two.txt: Found 3 duplicates - deleting identical copies
    two.txt: 3 duplicates remain
      1:       564374 bytes, 2016-03-05 16:22:52.118000000, md5sum 7594e7dc9fc28f727c42ee3e0749de81
      2:      6048320 bytes, 2016-03-05 16:22:46.185000000, md5sum 1eedaa9fe86fd4b8632e2ac549403b36
      3:      1744073 bytes, 2016-03-05 16:22:38.104000000, md5sum 851957f7fb6f0bc4ce76be966d336802
    s) Skip and do nothing
    k) Keep just one (choose which in next step)
    r) Rename all to be different (by changing file.jpg to file-1.jpg)
    s/k/r> r
    two-1.txt: renamed from: two.txt
    two-2.txt: renamed from: two.txt
    two-3.txt: renamed from: two.txt

The result being

    $ rclone lsl drive:dupes
      6048320 2016-03-05 16:23:16.798000000 one.txt
       564374 2016-03-05 16:22:52.118000000 two-1.txt
      6048320 2016-03-05 16:22:46.185000000 two-2.txt
      1744073 2016-03-05 16:22:38.104000000 two-3.txt

Dedupe can be run non interactively using the ` + "`" + `--dedupe-mode` + "`" + ` flag or by using an extra parameter with the same value

  * ` + "`" + `--dedupe-mode interactive` + "`" + ` - interactive as above.
  * ` + "`" + `--dedupe-mode skip` + "`" + ` - removes identical files then skips anything left.
  * ` + "`" + `--dedupe-mode first` + "`" + ` - removes identical files then keeps the first one.
  * ` + "`" + `--dedupe-mode newest` + "`" + ` - removes identical files then keeps the newest one.
  * ` + "`" + `--dedupe-mode oldest` + "`" + ` - removes identical files then keeps the oldest one.
  * ` + "`" + `--dedupe-mode rename` + "`" + ` - removes identical files then renames the rest to be different.

For example to rename all the identically named photos in your Google Photos directory, do

    rclone dedupe --dedupe-mode rename "drive:Google Photos"

Or

    rclone dedupe rename "drive:Google Photos"
`,
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(1, 2, command, args)
		if len(args) > 1 {
			err := dedupeMode.Set(args[0])
			if err != nil {
				log.Fatal(err)
			}
			args = args[1:]
		}
		fdst := cmd.NewFsSrc(args)
		cmd.Run(false, false, command, func() error {
			return fs.Deduplicate(fdst, dedupeMode)
		})
	},
}
