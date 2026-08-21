"use client"

import { useCallback, useRef, useState, useEffect } from "react"
import dynamic from "next/dynamic"
import type { editor } from "monaco-editor"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Textarea } from "@/components/ui/textarea"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogFooter,
} from "@/components/ui/dialog"
import { MessageSquare, Bug, Lightbulb, HelpCircle } from "lucide-react"

// Monaco DiffEditor 动态导入（减少首屏体积）
const DiffEditor = dynamic(
  () => import("@monaco-editor/react").then((mod) => mod.DiffEditor),
  { ssr: false },
)

// ==================== 类型定义 ====================

export interface DiffLineComment {
  id: string
  lineNum: number
  content: string
  type: "defect" | "suggestion" | "question" | "praise"
  author: string
  createdAt: string
}

export interface DiffFile {
  path: string
  original: string
  modified: string
  language?: string
}

interface DiffViewerProps {
  file: DiffFile
  comments?: DiffLineComment[]
  onAddComment?: (lineNum: number, content: string, type: DiffLineComment["type"]) => void
  onResolveComment?: (commentId: string) => void
  loading?: boolean
}

// ==================== 语言映射 ====================

const EXT_TO_LANG: Record<string, string> = {
  ts: "typescript",
  tsx: "typescript",
  js: "javascript",
  jsx: "javascript",
  go: "go",
  rs: "rust",
  py: "python",
  java: "java",
  rb: "ruby",
  php: "php",
  cpp: "cpp",
  c: "c",
  h: "c",
  cs: "csharp",
  swift: "swift",
  kt: "kotlin",
  scala: "scala",
  sql: "sql",
  yaml: "yaml",
  yml: "yaml",
  json: "json",
  xml: "xml",
  html: "html",
  css: "css",
  scss: "scss",
  less: "less",
  md: "markdown",
  sh: "shell",
  bash: "shell",
  dockerfile: "dockerfile",
}

function detectLanguage(filePath: string): string {
  const ext = filePath.split(".").pop()?.toLowerCase() || ""
  return EXT_TO_LANG[ext] || "plaintext"
}

// ==================== 评论标记图标 ====================

const TYPE_ICONS: Record<string, React.ReactNode> = {
  defect: <Bug className="h-3 w-3 text-red-500" />,
  suggestion: <Lightbulb className="h-3 w-3 text-amber-500" />,
  question: <HelpCircle className="h-3 w-3 text-blue-500" />,
  praise: <MessageSquare className="h-3 w-3 text-green-500" />,
}

const TYPE_LABELS: Record<string, string> = {
  defect: "缺陷",
  suggestion: "建议",
  question: "疑问",
  praise: "赞扬",
}

// ==================== 主组件 ====================

export function DiffViewer({
  file,
  comments = [],
  onAddComment,
  onResolveComment,
  loading,
}: DiffViewerProps) {
  const editorRef = useRef<editor.IStandaloneDiffEditor | null>(null)
  const [selectedLine, setSelectedLine] = useState<number | null>(null)
  const [commentOpen, setCommentOpen] = useState(false)
  const [commentText, setCommentText] = useState("")
  const [commentType, setCommentType] = useState<DiffLineComment["type"]>("suggestion")

  const language = file.language || detectLanguage(file.path)

  // 按行分组评论
  const commentsByLine = useCallback(() => {
    const map = new Map<number, DiffLineComment[]>()
    for (const c of comments) {
      const existing = map.get(c.lineNum) || []
      existing.push(c)
      map.set(c.lineNum, existing)
    }
    return map
  }, [comments])

  // Monaco 编辑器挂载完成
  const handleEditorMount = useCallback((editor: editor.IStandaloneDiffEditor) => {
    editorRef.current = editor

    // 获取修改后编辑器的实例
    const modifiedEditor = editor.getModifiedEditor()

    // 点击行号触发评论
    modifiedEditor.onMouseDown((e: editor.IEditorMouseEvent) => {
      const target = e.target
      if (target.type === 2 || target.type === 3) { // GUTTER_GLYPH_MARGIN or GUTTER_LINE_NUMBERS
        const lineNum = target.position?.lineNumber
        if (lineNum) {
          setSelectedLine(lineNum)
          setCommentText("")
          setCommentType("suggestion")
          setCommentOpen(true)
        }
      }
    })
  }, [])

  // 提交评论
  const handleSubmitComment = () => {
    if (!commentText.trim() || !selectedLine || !onAddComment) return
    onAddComment(selectedLine, commentText.trim(), commentType)
    setCommentOpen(false)
    setCommentText("")
    setSelectedLine(null)
  }

  // 装饰器：标记有评论的行
  useEffect(() => {
    const ed = editorRef.current?.getModifiedEditor()
    if (!ed) return

    const decorations = commentsByLine()
    const markers: editor.IModelDeltaDecoration[] = []

    decorations.forEach((lineComments, lineNum) => {
      const hasDefect = lineComments.some((c) => c.type === "defect")
      const color = hasDefect ? "#ef4444" : "#f59e0b"
      markers.push({
        range: {
          startLineNumber: lineNum,
          startColumn: 1,
          endLineNumber: lineNum,
          endColumn: 1,
        },
        options: {
          isWholeLine: true,
          linesDecorationsClassName: "diff-comment-marker",
          glyphMarginClassName: `diff-glyph-${hasDefect ? "defect" : "comment"}`,
          glyphMarginHoverMessage: {
            value: `${lineComments.length} 条评论${hasDefect ? "（含缺陷标记）" : ""}`,
          },
        },
      })
    })

    ed.createDecorationsCollection(markers)
  }, [comments, commentsByLine])

  // 文件行内评论展示
  const lineComments = selectedLine ? commentsByLine().get(selectedLine) || [] : []

  return (
    <div className="space-y-2">
      {/* 文件头 */}
      <div className="flex items-center justify-between rounded-t-lg border bg-muted/30 px-3 py-2">
        <div className="flex items-center gap-2">
          <span className="text-sm font-mono">{file.path}</span>
          <Badge variant="outline" className="text-xs">{language}</Badge>
          {comments.length > 0 && (
            <Badge variant="secondary" className="text-xs">
              {comments.length} 条评论
            </Badge>
          )}
        </div>
      </div>

      {/* Monaco Diff 编辑器 */}
      <div className="rounded-b-lg border overflow-hidden">
        {loading ? (
          <div className="flex h-64 items-center justify-center text-sm text-muted-foreground">
            加载 Diff 中...
          </div>
        ) : (
          <DiffEditor
            original={file.original}
            modified={file.modified}
            language={language}
            theme="vs"
            onMount={handleEditorMount}
            options={{
              renderSideBySide: true,
              readOnly: true,
              fontSize: 13,
              lineNumbers: "on",
              minimap: { enabled: false },
              scrollBeyondLastLine: false,
              wordWrap: "on",
              automaticLayout: true,
              glyphMargin: true,
              folding: false,
              lineDecorationsWidth: 8,
              lineNumbersMinChars: 3,
              diffAlgorithm: "advanced",
              // 差异高亮
              renderIndicators: true,
              // 仅显示有变更的区域（可折叠）
              hideUnchangedRegions: {
                enabled: true,
                minimumLineCount: 5,
                revealLineCount: 3,
              },
            }}
            height="500px"
          />
        )}
      </div>

      {/* 浮动评论对话框 */}
      <Dialog open={commentOpen} onOpenChange={setCommentOpen}>
        <DialogContent className="sm:max-w-md">
          <DialogHeader>
            <DialogTitle>
              行 {selectedLine} 添加评论
            </DialogTitle>
          </DialogHeader>

          {/* 已有评论 */}
          {lineComments.length > 0 && (
            <div className="max-h-32 space-y-2 overflow-y-auto rounded-md border p-2">
              {lineComments.map((c) => (
                <div key={c.id} className="flex items-start gap-2 text-sm">
                  <span className="mt-0.5 shrink-0">{TYPE_ICONS[c.type]}</span>
                  <div className="flex-1">
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{c.author}</span>
                      <Badge variant="outline" className="text-[10px] px-1">
                        {TYPE_LABELS[c.type]}
                      </Badge>
                    </div>
                    <p className="text-muted-foreground">{c.content}</p>
                  </div>
                  {onResolveComment && (
                    <Button
                      variant="ghost"
                      size="sm"
                      className="h-5 text-xs"
                      onClick={() => onResolveComment(c.id)}
                    >
                      解决
                    </Button>
                  )}
                </div>
              ))}
            </div>
          )}

          {/* 新建评论 */}
          {onAddComment && (
            <>
              <div className="space-y-2">
                <Select
                  value={commentType}
                  onValueChange={(v) => setCommentType(v as DiffLineComment["type"])}
                >
                  <SelectTrigger>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="defect">
                      <span className="flex items-center gap-2">
                        <Bug className="h-3 w-3 text-red-500" /> 缺陷
                      </span>
                    </SelectItem>
                    <SelectItem value="suggestion">
                      <span className="flex items-center gap-2">
                        <Lightbulb className="h-3 w-3 text-amber-500" /> 建议
                      </span>
                    </SelectItem>
                    <SelectItem value="question">
                      <span className="flex items-center gap-2">
                        <HelpCircle className="h-3 w-3 text-blue-500" /> 疑问
                      </span>
                    </SelectItem>
                    <SelectItem value="praise">
                      <span className="flex items-center gap-2">
                        <MessageSquare className="h-3 w-3 text-green-500" /> 赞扬
                      </span>
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <Textarea
                value={commentText}
                onChange={(e) => setCommentText(e.target.value)}
                placeholder="输入评论内容..."
                rows={3}
              />
              <DialogFooter>
                <Button variant="outline" onClick={() => setCommentOpen(false)}>
                  取消
                </Button>
                <Button onClick={handleSubmitComment} disabled={!commentText.trim()}>
                  提交
                </Button>
              </DialogFooter>
            </>
          )}
        </DialogContent>
      </Dialog>

      {/* 全局样式 - 评论标记 */}
      <style jsx global>{`
        .diff-comment-marker {
          background-color: rgba(251, 191, 36, 0.1);
          border-left: 3px solid #f59e0b;
        }
        .diff-glyph-comment {
          background: #f59e0b;
          border-radius: 50%;
          width: 8px !important;
          height: 8px !important;
          margin-left: 6px;
        }
        .diff-glyph-defect {
          background: #ef4444;
          border-radius: 50%;
          width: 8px !important;
          height: 8px !important;
          margin-left: 6px;
        }
      `}</style>
    </div>
  )
}
