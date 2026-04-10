declare module '@toast-ui/editor' {
  interface EditorOptions {
    el: HTMLElement;
    height?: string;
    minHeight?: string;
    initialEditType?: 'markdown' | 'wysiwyg';
    previewStyle?: 'tab' | 'vertical';
    initialValue?: string;
    usageStatistics?: boolean;
    placeholder?: string;
    theme?: 'light' | 'dark';
    hideModeSwitch?: boolean;
    language?: string;
    useCommandShortcut?: boolean;
    toolbarItems?: Array<Array<string>>;
    hooks?: {
      addImageBlobHook?: (blob: Blob | File, callback: (url: string, text?: string) => void) => void;
    };
    events?: {
      load?: () => void;
      change?: () => void;
      caretChange?: () => void;
      blur?: () => void;
      focus?: () => void;
    };
  }

  class Editor {
    constructor(options: EditorOptions);
    getMarkdown(): string;
    setMarkdown(markdown: string, cursorToEnd?: boolean): void;
    getHTML(): string;
    setHTML(html: string, cursorToEnd?: boolean): void;
    insertText(text: string): void;
    moveCursorToEnd(): void;
    moveCursorToStart(): void;
    focus(): void;
    blur(): void;
    disable(): void;
    enable(): void;
    reset(): void;
    destroy(): void;
    hide(): void;
    show(): void;
    changeMode(mode: 'markdown' | 'wysiwyg'): void;
    changePreviewStyle(style: 'tab' | 'vertical'): void;
    setHeight(height: string): void;
    on(event: string, callback: (...args: any[]) => void): void;
    off(event: string): void;
  }

  export default Editor;
}
