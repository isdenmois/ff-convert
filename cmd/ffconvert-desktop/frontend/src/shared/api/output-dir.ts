import { GetOutputDirInfo, ResetOutputDir, SetOutputDir } from "../../../wailsjs/go/main/App";

export interface OutputDirInfo {
  current: string;
  default: string;
}

/** Current output folder plus the startup default it can be reset to. */
export async function getOutputDirInfo(): Promise<OutputDirInfo> {
  return GetOutputDirInfo();
}

export const setOutputDir = SetOutputDir;
export const resetOutputDir = ResetOutputDir;
