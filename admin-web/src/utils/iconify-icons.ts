import antDesign from '@iconify/json/json/ant-design.json';
import carbon from '@iconify/json/json/carbon.json';
import ic from '@iconify/json/json/ic.json';
import lineMd from '@iconify/json/json/line-md.json';
import majesticons from '@iconify/json/json/majesticons.json';
import mdi from '@iconify/json/json/mdi.json';
import ph from '@iconify/json/json/ph.json';
import iconParkOutline from '@iconify/json/json/icon-park-outline.json';

interface IconifyJSON {
  prefix: string;
  icons: Record<string, unknown>;
}

export const iconifyCollections = [
  { name: 'Ant Design', prefix: 'ant-design', data: antDesign },
  { name: 'Carbon', prefix: 'carbon', data: carbon },
  { name: 'Iconify Icons', prefix: 'ic', data: ic },
  { name: 'Line MD', prefix: 'line-md', data: lineMd },
  { name: 'Majesticons', prefix: 'majesticons', data: majesticons },
  { name: 'Material Design', prefix: 'mdi', data: mdi },
  { name: 'Phosphor', prefix: 'ph', data: ph },
  { name: 'IconPark', prefix: 'icon-park-outline', data: iconParkOutline }
];

function getIconNames(collection: IconifyJSON): string[] {
  const prefix = collection.prefix;
  return Object.keys(collection.icons).map(name => `${prefix}:${name}`);
}

const allIconifyIcons: string[] = iconifyCollections.flatMap(col => getIconNames(col.data as IconifyJSON));

export function getAllIconifyIcons(): string[] {
  return allIconifyIcons;
}

export function getGroupedIconifyIcons() {
  return iconifyCollections.map(col => ({
    name: col.name,
    prefix: col.prefix,
    icons: getIconNames(col.data as IconifyJSON)
  }));
}
