package cloudfs

import (
	"errors"
	"io"
	"os"
	"path"
	"strings"

	"github.com/pkg/sftp"
	"github.com/sirupsen/logrus"
	log "github.com/sirupsen/logrus"
	"gocloud.dev/blob"
)

var folderPlaceHolderName = "__sftp_folder_placeholder__.txt"
var folderPlaceHolderContents = []byte("Place Holder")

//CloudFs file-system-y thing that the Hanlders live on
type CloudFs struct {
	bucket *blob.Bucket
	logger *logrus.Entry
}

//New creates a CloudFs
func New(bucket *blob.Bucket, logger *logrus.Entry) *CloudFs {
	return &CloudFs{
		bucket: bucket,
		logger: logger,
	}
}

//Fileread handles sftp file read requests
func (fs *CloudFs) Fileread(req *sftp.Request) (io.ReaderAt, error) {
	fs.logger.WithFields(log.Fields{
		"path": req.Filepath,
	}).Info("Beginning FileRead request")

	return &remoteFile{
		path:   req.Filepath,
		bucket: fs.bucket,
		ctx:    req.Context(),
	}, nil
}

//Filewrite handles sftp file write requests
func (fs *CloudFs) Filewrite(req *sftp.Request) (io.WriterAt, error) {
	fs.logger.WithFields(log.Fields{
		"path": req.Filepath,
	}).Info("Beginning FileWrite request")
	return newRemoteFileWriter(req.Context(), fs.bucket, req.Filepath)
}

//Filecmd handles sftp file cmd requests
func (fs *CloudFs) Filecmd(req *sftp.Request) error {
	logger := fs.logger.WithFields(log.Fields{
		"path":   req.Filepath,
		"target": req.Target,
		"method": req.Method,
	})
	logger.Info("Beginning FileCommand request")
	switch req.Method {
	case "Setstat":
		return nil
	case "Rename":
		err := fs.bucket.Copy(req.Context(), req.Target, req.Filepath, nil)
		if err != nil {
			logger.Error(err)
			return errors.New("Rename Failed")
		}
		err = fs.bucket.Delete(req.Context(), req.Filepath)
		if err != nil {
			logger.Error(err)
			return errors.New("Rename Failed")
		}
		return nil
	case "Rmdir":
		prefix := ""
		if req.Filepath != "/" {
			prefix = strings.TrimPrefix(req.Filepath, "/")
			if !strings.HasSuffix(prefix, "/") {
				prefix = prefix + "/"
			}
			logger.Debug(prefix)
		}

		listOptions := &blob.ListOptions{
			Prefix:    prefix,
			Delimiter: "/",
		}
		iter := fs.bucket.List(listOptions)
		for {
			obj, err := iter.Next(req.Context())

			if err == io.EOF {
				break
			}

			if err != nil {
				logger.Error(err)
				return err
			}

			err = fs.bucket.Delete(req.Context(), obj.Key)
			if err != nil {
				return errors.New("Failed to delete file: " + obj.Key)
			}
		}
	case "Remove":
		err := fs.bucket.Delete(req.Context(), req.Filepath)
		if err != nil {
			logger.Error(err)
			return errors.New("Remove Failed")
		}
	case "Mkdir":
		return fs.bucket.WriteAll(req.Context(), path.Join(req.Filepath, folderPlaceHolderName), folderPlaceHolderContents, nil)
	case "Link":
		return errors.New("SymLinks not supported")
	case "Symlink":
		return errors.New("SymLinks not supported")
	}
	return nil
}

//Filelist handles sftp file list requests
func (fs *CloudFs) Filelist(req *sftp.Request) (sftp.ListerAt, error) {
	logger := fs.logger.WithFields(log.Fields{
		"path":   req.Filepath,
		"method": req.Method,
	})
	logger.Info("Beginning FileList request")
	switch req.Method {
	case "List":
		prefix := ""
		if req.Filepath != "/" {
			prefix = strings.TrimPrefix(req.Filepath, "/")
			if !strings.HasSuffix(prefix, "/") {
				prefix = prefix + "/"
			}
			logger.Debug(prefix)
		}
		iter := fs.bucket.List(&blob.ListOptions{
			Prefix:    prefix,
			Delimiter: "/",
		})

		listObjects := []os.FileInfo{}
		for {
			obj, err := iter.Next(req.Context())

			if err == io.EOF {
				break
			}

			if err != nil {
				logger.Error(err)
				return nil, err
			}

			if strings.HasSuffix(obj.Key, folderPlaceHolderName) {
				continue
			}
			logger.Debug("ListResult: " + obj.Key)
			key := obj.Key
			if !strings.HasPrefix(key, "/") {
				key = "/" + obj.Key
			}
			listObjects = append(listObjects, &blobFileInfo{
				key:     key,
				modTime: obj.ModTime,
				size:    obj.Size,
				md5:     obj.MD5,
				isDir:   obj.IsDir,
			})
		}

		return listerat(listObjects), nil
	case "Stat":
		file := &remoteFile{
			bucket: fs.bucket,
			path:   req.Filepath,
			ctx:    req.Context(),
		}

		attrs, err := file.Attributes()
		if err != nil {
			logger.Error(err)
			ext := path.Ext(req.Filepath)
			if len(ext) == 0 {
				file = &remoteFile{
					bucket: fs.bucket,
					path:   path.Join(req.Filepath, folderPlaceHolderName),
					ctx:    req.Context(),
				}

				attrs, err := file.Attributes()
				if err != nil {
					return nil, errors.New("stat failed")
				}

				return listerat([]os.FileInfo{&blobFileInfo{
					key:     req.Filepath,
					modTime: attrs.ModTime,
					size:    0,
					isDir:   true,
				}}), nil
			}
			return nil, errors.New("stat failed")
		}

		return listerat([]os.FileInfo{&blobFileInfo{
			key:     req.Filepath,
			modTime: attrs.ModTime,
			size:    attrs.Size,
			md5:     attrs.MD5,
			isDir:   false,
		}}), nil
	case "Readlink":
		return nil, errors.New("symlinks not supported")
	}
	return nil, nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	var n int
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}
	n = copy(ls, f[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}
