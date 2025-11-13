# © ООО «1С-Софт», 2023.  Все права защищены
# Copyright © 2023, 1C-Soft LLC. All rights  reserved

from yaml import safe_load as yaml_safe_load, dump as yaml_dump, YAMLError
from io import open as io_open, TextIOWrapper
from os import path, getenv
from sys import platform
from datetime import datetime
from subprocess import Popen, CalledProcessError
from json import loads as json_loads, dumps as json_dumps
from urllib import parse as url_parse

from socket import getfqdn
from requests import post as post_request

from opensearchpy import OpenSearch

import subprocess

class MainSettings:
    def __init__(self) -> None:
        self.__settings = {}

    def open(self, filename):
        with open(filename, 'r') as stream:
            try:
                self.__settings = yaml_safe_load(stream)

                return True
            except YAMLError as exc:
                print(exc)

        return False

    def get(self, key):
        current_value = self.__settings
        settings_path = key.split('/')

        for part in settings_path:
            if part in current_value:
                current_value = current_value[part]
            else:
                return None

        return current_value
# --------------------------------------------------------------------------------------------------------------------------


class ProcessWatcher:

    def begin_watch(self, settings, sender):
        self.__data_sender = sender
        self.__ibcmd_exec = settings.get('paths/ibcmd_exec')
        self.__event_log_dir = settings.get('paths/event_log_dir')
        self.__format = settings.get('format')
        self.__follow_time = settings.get('follow-time-msec')

        infobase_id = settings.get('environments/infobase-id')

        if infobase_id:
            self.__infobase_id = infobase_id
        else:
            self.__infobase_id = self.__get_infobase_id()

        self.__ib_filename = f'{self.__infobase_id}.dat'

        module = settings.get('environments/module')
        metamodel_version = settings.get('environments/metamodel-version')

        service_name = settings.get('environments/service-name')
        service_version = settings.get('environments/service-version')

        if not service_name:
            service_name = getenv('E1C_SERVICE_NAME')

        if not service_version:
            service_version = getenv('E1C_SERVICE_VERSION')

        self.__module = module
        self.__metamodel_version = metamodel_version
        self.__service_name = service_name
        self.__service_version = service_version

        self.__watch_process()

    def __prepare_data_to_write(self, checkpoint=''):
        return {'checkpoint': checkpoint}

    def __write_checkpoint(self, data_to_write):

        with io_open(self.__ib_filename, 'w') as out_file:
            yaml_dump(data_to_write, out_file,
                      default_flow_style=False, allow_unicode=True)

    def __read_checkpoint(self):
        if not path.exists(self.__ib_filename):
            data_to_write = self.__prepare_data_to_write()
            self.__write_checkpoint(data_to_write)

            return data_to_write['checkpoint']

        with open(self.__ib_filename, 'r') as stream:
            try:
                data = yaml_safe_load(stream)

                return data['checkpoint'] if data else ''
            except YAMLError as exc:
                print(exc)

    def __get_infobase_id(self):
        splitted_path = path.normpath(self.__event_log_dir).split(path.sep)

        if splitted_path[-1].casefold() != '1Cv8Log'.casefold():
            raise Exception(f'"{self.__event_log_dir}" is not event log dir')

        return splitted_path[-2]

    def __watch_process(self):
        try:
            for line in self.__run_process():
                encode_page = 'utf8'

                if platform == "win32":
                    encode_page = 'cp866'

                if not line:
                    continue

                decoded_line = line.decode(encode_page)
                event_date = self.__data_sender.send(decoded_line,
                                                     self.__infobase_id,
                                                     self.__module,
                                                     self.__metamodel_version,
                                                     self.__service_name,
                                                     self.__service_version)

                data_to_write = self.__prepare_data_to_write(event_date)
                self.__write_checkpoint(data_to_write)

        except CalledProcessError as exception:
            print(exception)
            self.__watch_process()

    def __run_process(self):
        self.__checkpoint = self.__read_checkpoint()

        command_data = self.__build_process_command()
        ibcmd_process = Popen(command_data, stdout=subprocess.PIPE,
                              stderr=subprocess.PIPE,
                              universal_newlines=False, 
                              stdin=subprocess.PIPE)

        for stdout_line in iter(ibcmd_process.stdout.readline, ""):
            yield stdout_line

        error = []
        return_code = 0

        for stderr_line in iter(ibcmd_process.stderr.readline, ""):
            return_code = 1
            error.append(stderr_line)

        ibcmd_process.stderr.close()
        ibcmd_process.stdout.close()

        if return_code == 0:
            return_code = ibcmd_process.wait()

        if return_code:
            raise CalledProcessError(return_code, command_data)

    def __build_process_command(self):
        command_data = [f'{self.__ibcmd_exec}',
                        'eventlog',
                        'export',
                        '--format',
                        self.__format,
                        '--follow',
                        str(self.__follow_time),
                        '--skip-root']

        if self.__checkpoint != '':
            command_data.append('--from')
            command_data.append(self.__checkpoint)

        command_data.append(self.__event_log_dir)

        return command_data
# --------------------------------------------------------------------------------------------------------------------------


class AbstractDataSender:
    def __init__(self, settings) -> None:
        self.__event_data = {}

    def start(self):
        pass

    def send(self, data, id, service_name, service_version):
        pass
# --------------------------------------------------------------------------------------------------------------------------


class OpenSearchDataSender(AbstractDataSender):
    def __init__(self, settings) -> None:
        super().__init__(settings)

        self.__client = None
        self.__host = settings.get('receiver-parameters/opensearch/host')
        self.__port = settings.get('receiver-parameters/opensearch/port')
        self.__login = settings.get('receiver-parameters/opensearch/login')
        self.__password = settings.get('receiver-parameters/opensearch/password')
        self.__index = settings.get('receiver-parameters/opensearch/index')

    def start(self):
        auth = (self.__login, self.__password)

        self.__client = OpenSearch(
            hosts = [{'host': self.__host, 'port': self.__port}],
            http_compress = True,
            http_auth = auth,
            use_ssl = False,
            verify_certs = False,
            ssl_assert_hostname = False,
            ssl_show_warn = False)
        
        if not self.__client.indices.exists(self.__index):
            index_body = {
                'settings': {
                    'index': {
                        'number_of_shards': 4
                    }
                }
            }

            response = self.__client.indices.create(self.__index, body=index_body)
            print('\nCreating index:')
            print(response)

    def __encode_values(self, dict, key):
        value = dict[key]

        if isinstance(value, type(dict)):
            dict[key] = json_dumps(value, ensure_ascii=False)

    def send(self, input, db_id, service_name, service_version):
        event_data = json_loads(input)
        event_date = event_data['Date']
        
        params = {}
        excepted_keys = ['Event', 'EventPresentation', 'Session', 'User', 'UserName', 'Computer']

        for current_key in event_data.keys():
            if current_key in excepted_keys:
                continue

            self.__encode_values(event_data, current_key)

            current_value =  event_data[current_key]

            if current_value:
                params[current_key] = current_value
        
        try:
            current_date = datetime.strptime(event_date, '%Y-%m-%dT%H:%M:%S')
            timestamp = int(datetime.timestamp(current_date))
        except:
            return event_date

        document = {
            'ts': current_date,
            'tags': [db_id],
            'Datetime': timestamp,
            'serviceName': service_name if service_name else '',
            'seviceVersion': service_version if service_version else '',
            'name': event_data['EventPresentation'],
            'params': params,
            'sessionID': event_data['Session'] if 'Session' in event_data else '',
            'userLogin': event_data['User'],
            'userName': event_data['UserName'],
            'userNode': event_data['Computer'] if 'Computer' in event_data else ''
        }

        self.__client.index(
            index = self.__index,
            body = document,
            refresh = True
        )

        return event_date
# --------------------------------------------------------------------------------------------------------------------------


class HttpRequestDataSender(AbstractDataSender):
    def __init__(self, settings) -> None:
        super().__init__(settings)

        self.__url = settings['url']
        self.__node = ''

    def __encode_values(self, dict, key):
        value = dict[key]

        if isinstance(value, type(dict)):
            dict[key] = json_dumps(value, ensure_ascii=False)

    def start(self):
        self.__node = getfqdn()

    def send(self, input, db_id, module, metamodel_version, service_name, service_version):
        event_data = json_loads(input)
        event_date = event_data['Date']

        params = []
        excepted_keys = ['Event', 'EventPresentation',
                         'Session', 'User', 'UserName', 'Computer']

        for current_key in event_data.keys():
            if current_key in excepted_keys:
                continue

            self.__encode_values(event_data, current_key)

            current_value = event_data[current_key]

            if current_value:
                current_params = {'name': current_key, 'value': current_value}
                params.append(current_params)

        try:
            current_date = datetime.strptime(event_date, '%Y-%m-%dT%H:%M:%S')
            timestamp = int(datetime.timestamp(current_date))
        except Exception:
            return event_date

        session_id = event_data['Session'] if 'Session' in event_data else ''
        computer = event_data['Computer'] if 'Computer' in event_data else ''

        document = {
            'tags': [db_id],
            'createdAt': timestamp * 1000,
            'metamodelVersion': metamodel_version,
            'module': module,
            'name': event_data['Event'],
            'params': params,
            'session': session_id,
            'userLogin': event_data['UserName'],
            'userNode': computer
        }

        #print(json_dumps(document))

        headers = {'Content-type': 'application/json',
                   'X-Node-ID': self.__node}
        response = post_request(self.__url,
                                json=document,
                                headers=headers,
                                verify=True)


        if response.status_code != 201:
            print(f'Request has been completed with code: {response.status_code}')
            error_text = f'\nDescription: {response.text}' \
                         if response.text else ''

            print(f'Reason: {response.reason}{error_text}')

        return event_date
# --------------------------------------------------------------------------------------------------------------------------


class DataSenderCreator():
    @staticmethod
    def create(type, settings):
        settings_section = f'receiver-parameters/{type}'
        if type == 'opensearch':
            return OpenSearchDataSender(settings)
        
        elif  type == 'http-service':
            return HttpRequestDataSender(settings.get(settings_section))

        raise Exception(f'Unknown data sender: {type}')


# --------------------------------------------------------------------------------------------------------------------------
if __name__ == '__main__':
    settings = MainSettings()

    if not settings.open('settings.yml'):
        exit(1)

    current_receiver = settings.get('data-receiver')

    current_sender = DataSenderCreator.create(current_receiver, settings)
    current_sender.start()

    process_watcher = ProcessWatcher()
    process_watcher.begin_watch(settings, current_sender)
