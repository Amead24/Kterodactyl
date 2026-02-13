import { useCallback, useState } from 'react';
import { useDropzone } from 'react-dropzone';
import { Upload } from 'lucide-react';
import { toast } from 'sonner';
import { Progress } from '@/components/ui/progress';
import { useUploadMod } from '@/hooks/use-mods';

interface ModUploadProps {
  serverName: string;
}

export function ModUpload({ serverName }: ModUploadProps) {
  const [progress, setProgress] = useState<number | null>(null);
  const uploadMutation = useUploadMod();

  const onDrop = useCallback((acceptedFiles: File[]) => {
    if (acceptedFiles.length === 0) return;

    // Upload files sequentially
    const uploadNext = async (index: number) => {
      if (index >= acceptedFiles.length) {
        setProgress(null);
        toast.success(`${acceptedFiles.length} mod(s) uploaded. Server restarting...`);
        return;
      }

      const file = acceptedFiles[index];
      setProgress(0);

      try {
        await uploadMutation.mutateAsync({
          name: serverName,
          file,
          onProgress: setProgress,
        });
        await uploadNext(index + 1);
      } catch (err) {
        setProgress(null);
        toast.error(`Failed to upload ${file.name}: ${err instanceof Error ? err.message : 'Unknown error'}`);
      }
    };

    uploadNext(0);
  }, [serverName, uploadMutation]);

  const { getRootProps, getInputProps, isDragActive } = useDropzone({ onDrop });

  return (
    <div className="space-y-3">
      <div
        {...getRootProps()}
        className={`flex cursor-pointer flex-col items-center justify-center rounded-lg border-2 border-dashed p-8 transition-colors ${
          isDragActive
            ? 'border-primary bg-primary/5'
            : 'border-muted-foreground/25 hover:border-primary/50'
        }`}
      >
        <input {...getInputProps()} />
        <Upload className="mb-2 size-8 text-muted-foreground" />
        {isDragActive ? (
          <p className="text-sm text-primary">Drop files here...</p>
        ) : (
          <p className="text-sm text-muted-foreground">
            Drag & drop mod files here, or click to browse
          </p>
        )}
        <p className="mt-1 text-xs text-muted-foreground">Max 100MB per file</p>
      </div>
      {progress !== null && (
        <div className="space-y-1">
          <Progress value={progress} className="h-2" />
          <p className="text-xs text-muted-foreground text-center">{progress}%</p>
        </div>
      )}
    </div>
  );
}
