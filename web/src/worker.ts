/**
 * Worker 薄壳：把 self.onmessage 接到 Pipeline；重活都在 lib/pipeline.ts。
 */
import { Pipeline } from "./lib/pipeline";
import { encodeGif, encodePng } from "./lib/imaging";
import type { WorkerRequest, WorkerResponse } from "./lib/protocol";

const pipeline = new Pipeline(
  (msg: WorkerResponse) => (self as unknown as Worker).postMessage(msg),
  { encodePng, encodeGif },
);

self.onmessage = async (ev: MessageEvent<WorkerRequest>) => {
  await pipeline.dispatch(ev.data);
};
