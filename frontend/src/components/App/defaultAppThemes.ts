/*
 * Copyright 2025 The Kubernetes Authors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

import { AppTheme } from '../../lib/AppTheme';

export const classicLightTheme: AppTheme = {
  name: 'Classic Light',
  primary: '#222',
  secondary: '#eaeaea',
  sidebar: {
    background: '#242424',
    color: '#FFF',
    selectedBackground: '#ebe811',
    selectedColor: '#ebe811',
    actionBackground: '#605e5c',
  },
  link: {
    color: '#0072c9',
  },
  navbar: {
    background: '#FFF',
    color: '#202020',
  },
  radius: 4,
};

export const darkTheme: AppTheme = {
  name: 'Dark',
  base: 'dark',
  primary: '#3B82F6',
  secondary: '#1E293B',
  text: {
    primary: '#F1F5F9',
  },
  link: {
    color: '#60A5FA',
  },
  background: {
    default: '#0F172A',
    surface: '#1E293B',
    muted: '#1E293B',
  },
  navbar: {
    background: '#0F172A',
    color: '#F1F5F9',
  },
  sidebar: {
    background: '#0F172A',
    color: '#CBD5E1',
    selectedBackground: '#3B82F6',
    selectedColor: '#FFFFFF',
    actionBackground: '#1E293B',
  },
  buttonTextTransform: 'none',
  radius: 8,
  fontFamily: ['Inter Variable', 'Inter', 'sans-serif'],
};

export const lightTheme: AppTheme = {
  name: 'Light',
  primary: '#0F172A',
  secondary: '#F8FAFC',
  text: {
    primary: '#0F172A',
  },
  link: {
    color: '#3B82F6',
  },
  background: {
    default: '#FFFFFF',
    surface: '#FFFFFF',
    muted: '#F8FAFC',
  },
  sidebar: {
    background: '#0F172A',
    color: '#CBD5E1',
    selectedBackground: '#3B82F6',
    selectedColor: '#FFFFFF',
    actionBackground: '#1E293B',
  },
  navbar: {
    background: '#F8FAFC',
    color: '#0F172A',
  },
  buttonTextTransform: 'none',
  radius: 8,
  fontFamily: ['Inter Variable', 'Inter', 'sans-serif'],
};

export const lightsOutTheme: AppTheme = {
  name: 'Lights Out',
  base: 'dark',
  primary: '#1f6feb',
  secondary: '#212830',
  text: {
    primary: '#f0f6fc',
  },
  link: {
    color: '#4493f8',
  },
  background: {
    default: '#010409',
    surface: '#0d1117',
    muted: '#151b23',
  },
  sidebar: {
    background: '#010409',
    color: '#f0f6fc',
    selectedBackground: '#484f57',
    selectedColor: '#fff',
    actionBackground: '#1f6feb',
  },
  navbar: {
    background: '#010409',
    color: '#bdc3c9',
  },
  radius: 6,
  buttonTextTransform: 'none',
};

export const monochromeLightTheme: AppTheme = {
  name: 'Monochrome Light',
  base: 'light',
  primary: '#25292e',
  secondary: '#f6f8fa',
  text: {
    primary: '#1f2328',
  },
  link: {
    color: '#0969da',
  },
  background: {
    default: '#ffffff',
    surface: '#ffffff',
    muted: '#f6f8fa',
  },
  sidebar: {
    background: '#fff',
    color: '#59636e',
    selectedBackground: '#333',
    selectedColor: '#1f2328',
    actionBackground: '#333436',
  },
  navbar: {
    background: '#ffffff',
    color: '#1f2328',
  },
  radius: 6,
  buttonTextTransform: 'none',
};

const defaultAppThemes = [
  lightTheme,
  darkTheme,
  classicLightTheme,
  lightsOutTheme,
  monochromeLightTheme,
];

export default defaultAppThemes;
