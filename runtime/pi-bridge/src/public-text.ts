// Public assistant text only. Reasoning and tool argument blocks never enter
// this projection. Completion fills missing deltas rather than duplicating them.
export class PublicText {
  private message = 0;
  private blocks = new Map<number, string>();
  constructor(
    private send: (frame: {
      type: "text_delta";
      id: string;
      text: string;
    }) => void,
  ) {}
  start() {
    this.message++;
    this.blocks.clear();
  }
  delta(index: number, text: string) {
    this.blocks.set(index, (this.blocks.get(index) ?? "") + text);
    if (text)
      this.send({ type: "text_delta", id: `message-${this.message}`, text });
  }
  complete(index: number, text: string) {
    const previous = this.blocks.get(index) ?? "";
    if (!text.startsWith(previous)) throw new Error("model_text_mismatch");
    this.delta(index, text.slice(previous.length));
  }
}
