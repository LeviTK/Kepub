// Deliberately never read Go's startup/cancel pipe. The supervisor must still
// enforce its deadline even when the first 60 KiB write cannot drain.
setInterval(() => {}, 1000);
