import { addCollection } from '@iconify/vue';
import antDesign from '@iconify/json/json/ant-design.json';
import carbon from '@iconify/json/json/carbon.json';
import ic from '@iconify/json/json/ic.json';
import lineMd from '@iconify/json/json/line-md.json';
import majesticons from '@iconify/json/json/majesticons.json';
import mdi from '@iconify/json/json/mdi.json';
import ph from '@iconify/json/json/ph.json';
import iconParkOutline from '@iconify/json/json/icon-park-outline.json';

const collections = [
  { name: 'ant-design', data: antDesign },
  { name: 'carbon', data: carbon },
  { name: 'ic', data: ic },
  { name: 'line-md', data: lineMd },
  { name: 'majesticons', data: majesticons },
  { name: 'mdi', data: mdi },
  { name: 'ph', data: ph },
  { name: 'icon-park-outline', data: iconParkOutline }
];

export function setupIconifyOffline() {
  collections.forEach(collection => {
    addCollection(collection.data as any, collection.name);
  });
}
