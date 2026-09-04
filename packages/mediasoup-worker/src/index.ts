import cors from "cors";
import dotenv from "dotenv";
import express from "express";
import { createRouter } from "./api";
import MediasoupWorker from "./worker";

dotenv.config();

async function main(): Promise<void> {
	const worker = new MediasoupWorker();
	await worker.init();
	console.log("[mediasoup] worker started");

	const app = express();
	app.use(cors());
	app.use(express.json());
	app.use("/api", createRouter(worker));

	const port = Number(process.env.PORT || 3012);
	const server = app.listen(port, () => console.log(`[mediasoup] HTTP bridge on :${port}`));

	const shutdown = async () => {
		server.close();
		await worker.close();
		process.exit(0);
	};
	process.on("SIGINT", shutdown);
	process.on("SIGTERM", shutdown);
}

main().catch((err) => {
	console.error(err);
	process.exit(1);
});
