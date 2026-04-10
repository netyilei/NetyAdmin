import { defineStore } from 'pinia';
import { fetchGetDictData } from '@/service/api/v1/system-dict';
import type { SystemDict } from '@/typings/api/v1/system-dict';

interface DictState {
  dictMap: Map<string, SystemDict.DictData[]>;
}

export const useDictStore = defineStore('dict-store', {
  state: (): DictState => ({
    dictMap: new Map()
  }),
  actions: {
    async getDict(code: string) {
      if (this.dictMap.has(code)) {
        return this.dictMap.get(code)!;
      }

      const { data, error } = await fetchGetDictData(code);
      if (!error && data) {
        this.dictMap.set(code, data);
        return data;
      }
      return [];
    },
    async loadDicts(codes: string[]) {
      await Promise.all(codes.map(code => this.getDict(code)));
    },
    clearCache() {
      this.dictMap.clear();
    }
  }
});
